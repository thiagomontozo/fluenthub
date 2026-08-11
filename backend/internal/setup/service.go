package setup

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thiagomontozo/fluenthub/backend/internal/auth"
)

type Input struct {
	LegalName, DisplayName, Slug, Email, Phone, Website, Timezone, Locale string
	SystemTitle, PrimaryColor, SecondaryColor, AccentColor, WelcomeText   string
	AdministratorName, AdministratorEmail, AdministratorPassword          string
	UnitName, UnitCode                                                    string
	MinimumPassingScoreScaled, ExerciseWeightBasisPoints                  int
	ExamWeightBasisPoints, MinimumAttendanceBasisPoints                   int
}

type Service struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) Status(ctx context.Context) (bool, error) {
	var configured bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schools)`).Scan(&configured)
	return configured, err
}

func (s *Service) Complete(ctx context.Context, input Input) (string, error) {
	if err := validate(input); err != nil {
		return "", err
	}
	hash, err := auth.HashPassword(input.AdministratorPassword)
	if err != nil {
		return "", err
	}
	schoolAddress, _ := mail.ParseAddress(input.Email)
	administratorAddress, _ := mail.ParseAddress(input.AdministratorEmail)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(61822026)`); err != nil {
		return "", err
	}
	var configured bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schools)`).Scan(&configured); err != nil {
		return "", err
	}
	if configured {
		return "", errors.New("initial setup has already been completed")
	}
	schoolID, adminID, unitID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := tx.Exec(ctx, `INSERT INTO schools(id,legal_name,display_name,slug,email,phone,website,timezone,locale) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, schoolID, input.LegalName, input.DisplayName, strings.ToLower(input.Slug), strings.ToLower(schoolAddress.Address), input.Phone, input.Website, input.Timezone, input.Locale); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO users(id,school_id,name,email,password_hash) VALUES($1,$2,$3,$4,$5)`, adminID, schoolID, input.AdministratorName, strings.ToLower(administratorAddress.Address), hash); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO school_branding(school_id,system_title,school_display_name,primary_color,secondary_color,accent_color,welcome_text,updated_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, schoolID, input.SystemTitle, input.DisplayName, input.PrimaryColor, input.SecondaryColor, input.AccentColor, input.WelcomeText, adminID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO units(id,school_id,name,code,timezone) VALUES($1,$2,$3,$4,$5)`, unitID, schoolID, input.UnitName, input.UnitCode, input.Timezone); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO academic_policies(school_id,minimum_passing_score_scaled,exercise_weight_basis_points,exam_weight_basis_points,minimum_attendance_basis_points,updated_by) VALUES($1,$2,$3,$4,$5,$6)`, schoolID, input.MinimumPassingScoreScaled, input.ExerciseWeightBasisPoints, input.ExamWeightBasisPoints, input.MinimumAttendanceBasisPoints, adminID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO attendance_policies(school_id,minimum_live_attendance_basis_points,updated_by) VALUES($1,$2,$3)`, schoolID, input.MinimumAttendanceBasisPoints, adminID); err != nil {
		return "", err
	}
	roles := map[string]string{"administrator": "Administrator", "teacher": "Teacher", "operator": "Operator", "student": "Student"}
	for code, name := range roles {
		roleID := uuid.NewString()
		if _, err := tx.Exec(ctx, `INSERT INTO roles(id,school_id,name,code,system) VALUES($1,$2,$3,$4,true)`, roleID, schoolID, name, code); err != nil {
			return "", err
		}
		if code == "administrator" {
			if _, err := tx.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) SELECT $1,id FROM permissions`, roleID); err != nil {
				return "", err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO user_roles(user_id,role_id) VALUES($1,$2)`, adminID, roleID); err != nil {
				return "", err
			}
		} else if _, err := tx.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) SELECT $1,id FROM permissions WHERE code = ANY($2)`, roleID, defaultPermissions(code)); err != nil {
			return "", err
		}
	}
	for _, skill := range []string{"reading", "writing", "listening", "speaking", "grammar", "vocabulary"} {
		if _, err := tx.Exec(ctx, `INSERT INTO language_skills(school_id,code,name) VALUES($1,$2,$3)`, schoolID, skill, strings.ToUpper(skill[:1])+skill[1:]); err != nil {
			return "", err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO setup_progress(id,current_step,completed_at) VALUES(true,'finish',now()) ON CONFLICT(id) DO UPDATE SET current_step='finish',completed_at=now(),updated_at=now()`); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(school_id,user_id,action,resource_type,resource_id) VALUES($1,$2,'setup.completed','school',$1)`, schoolID, adminID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return schoolID, nil
}

func validate(input Input) error {
	if len(strings.TrimSpace(input.LegalName)) < 2 || len(strings.TrimSpace(input.DisplayName)) < 2 || !validSlug(input.Slug) || input.Timezone == "" || input.Locale == "" {
		return errors.New("school identity, timezone and locale are required")
	}
	if _, err := mail.ParseAddress(input.Email); err != nil {
		return errors.New("invalid school email")
	}
	if _, err := mail.ParseAddress(input.AdministratorEmail); err != nil {
		return errors.New("invalid administrator email")
	}
	if len(strings.TrimSpace(input.AdministratorName)) < 2 || len(strings.TrimSpace(input.UnitName)) < 2 || input.UnitCode == "" {
		return errors.New("administrator and first unit are required")
	}
	if input.ExerciseWeightBasisPoints+input.ExamWeightBasisPoints != 10000 {
		return errors.New("assessment weights must total 10000 basis points")
	}
	if input.MinimumPassingScoreScaled < 0 || input.MinimumPassingScoreScaled > 10000 || input.MinimumAttendanceBasisPoints < 0 || input.MinimumAttendanceBasisPoints > 10000 {
		return errors.New("invalid academic policy")
	}
	if strings.TrimSpace(input.SystemTitle) == "" || !validHexColor(input.PrimaryColor) || !validHexColor(input.SecondaryColor) || !validHexColor(input.AccentColor) {
		return errors.New("system title and valid branding colors are required")
	}
	return nil
}

func validSlug(value string) bool {
	if len(value) < 2 || len(value) > 80 {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-') {
			return false
		}
	}
	return true
}
func validHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, character := range value[1:] {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func defaultPermissions(role string) []string {
	switch role {
	case "teacher":
		return []string{"classes.read", "lessons.read", "lessons.manage", "exercises.read", "exercises.manage", "exercises.grade", "exams.read", "exams.manage", "exams.grade", "attendance.read", "attendance.manage", "academic.read", "notifications.read", "support.open"}
	case "operator":
		return []string{"users.read", "students.manage", "classes.read", "billing.read", "billing.manage", "notifications.read", "notifications.send", "support.open", "support.handle"}
	default:
		return []string{"classes.read", "lessons.read", "exercises.read", "exams.read", "attendance.read", "academic.read", "billing.read", "notifications.read", "support.open", "certificates.read"}
	}
}
