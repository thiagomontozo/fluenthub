package users

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thiagomontozo/fluenthub/backend/internal/auth"
)

type SessionUser struct {
	ID, SchoolID, Name, Email string
	Roles, Permissions        []string
}
type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }
func (s *Store) Authenticate(ctx context.Context, email, password string) (SessionUser, string, error) {
	var u SessionUser
	var hash string
	var active bool
	err := s.db.QueryRow(ctx, `SELECT id,school_id,name,email,password_hash,active FROM users WHERE lower(email)=lower($1)`, email).Scan(&u.ID, &u.SchoolID, &u.Name, &u.Email, &hash, &active)
	if err != nil || !active || !auth.CheckPassword(hash, password) {
		return SessionUser{}, "", errors.New("invalid credentials")
	}
	raw, digest, err := auth.NewToken()
	if err != nil {
		return SessionUser{}, "", err
	}
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	_, err = s.db.Exec(ctx, `INSERT INTO user_sessions(user_id,token_hash,expires_at) VALUES($1,$2,$3)`, u.ID, digest, expires)
	if err != nil {
		return SessionUser{}, "", err
	}
	_, _ = s.db.Exec(ctx, `UPDATE users SET last_login_at=now() WHERE id=$1`, u.ID)
	return s.withAccess(ctx, u), raw, nil
}
func (s *Store) ByToken(ctx context.Context, raw string) (SessionUser, error) {
	var u SessionUser
	err := s.db.QueryRow(ctx, `SELECT u.id,u.school_id,u.name,u.email FROM user_sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND u.active=true`, auth.Digest(raw)).Scan(&u.ID, &u.SchoolID, &u.Name, &u.Email)
	if err != nil {
		return SessionUser{}, err
	}
	return s.withAccess(ctx, u), nil
}
func (s *Store) Revoke(ctx context.Context, raw string) error {
	_, err := s.db.Exec(ctx, `UPDATE user_sessions SET revoked_at=now() WHERE token_hash=$1`, auth.Digest(raw))
	return err
}
func (s *Store) ChangePassword(ctx context.Context, userID, current, next string) error {
	var hash string
	if err := s.db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1 AND active=true`, userID).Scan(&hash); err != nil {
		return err
	}
	if !auth.CheckPassword(hash, current) {
		return errors.New("current password is invalid")
	}
	newHash, err := auth.HashPassword(next)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$1,updated_at=now() WHERE id=$2`, newHash, userID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE user_sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) withAccess(ctx context.Context, u SessionUser) SessionUser {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT r.code,p.code FROM user_roles ur JOIN roles r ON r.id=ur.role_id LEFT JOIN role_permissions rp ON rp.role_id=r.id LEFT JOIN permissions p ON p.id=rp.permission_id WHERE ur.user_id=$1`, u.ID)
	if err != nil {
		return u
	}
	defer rows.Close()
	seenRole := map[string]bool{}
	seenPerm := map[string]bool{}
	for rows.Next() {
		var role string
		var permission *string
		if rows.Scan(&role, &permission) == nil {
			if !seenRole[role] {
				u.Roles = append(u.Roles, role)
				seenRole[role] = true
			}
			if permission != nil && !seenPerm[*permission] {
				u.Permissions = append(u.Permissions, *permission)
				seenPerm[*permission] = true
			}
		}
	}
	return u
}
func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

type PublicUser struct {
	ID, Name, Email string
	Phone           *string
	Active          bool
	LastLoginAt     *time.Time
	Roles           []string
}
type CreateInput struct {
	Name, Email, Password, RoleCode string
	Phone                           *string
}

func (s *Store) List(ctx context.Context, schoolID, role string, limit, offset int) ([]PublicUser, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, errors.New("invalid pagination")
	}
	rows, err := s.db.Query(ctx, `SELECT u.id,u.name,u.email,u.phone,u.active,u.last_login_at,COALESCE(array_agg(r.code) FILTER(WHERE r.code IS NOT NULL),'{}') FROM users u LEFT JOIN user_roles ur ON ur.user_id=u.id LEFT JOIN roles r ON r.id=ur.role_id WHERE u.school_id=$1 AND ($4='' OR EXISTS(SELECT 1 FROM user_roles fur JOIN roles fr ON fr.id=fur.role_id WHERE fur.user_id=u.id AND fr.code=$4)) GROUP BY u.id ORDER BY u.name LIMIT $2 OFFSET $3`, schoolID, limit, offset, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PublicUser, 0)
	for rows.Next() {
		var item PublicUser
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Phone, &item.Active, &item.LastLoginAt, &item.Roles); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListTeacherStudents(ctx context.Context, schoolID, teacherID string, limit, offset int) ([]PublicUser, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, errors.New("invalid pagination")
	}
	rows, err := s.db.Query(ctx, `SELECT DISTINCT u.id,u.name,u.email,u.phone,u.active,u.last_login_at,ARRAY['student']::text[] FROM users u JOIN enrollments e ON e.student_id=u.id JOIN class_groups c ON c.id=e.class_id WHERE u.school_id=$1 AND c.teacher_id=$2 AND e.status IN ('active','suspended') ORDER BY u.name LIMIT $3 OFFSET $4`, schoolID, teacherID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PublicUser, 0)
	for rows.Next() {
		var item PublicUser
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Phone, &item.Active, &item.LastLoginAt, &item.Roles); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) Create(ctx context.Context, schoolID, actorID string, input CreateInput) (PublicUser, error) {
	if len(strings.TrimSpace(input.Name)) < 2 {
		return PublicUser{}, errors.New("name is required")
	}
	address, err := mail.ParseAddress(input.Email)
	if err != nil {
		return PublicUser{}, errors.New("invalid email")
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return PublicUser{}, err
	}
	id := uuid.NewString()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return PublicUser{}, err
	}
	defer tx.Rollback(ctx)
	var roleID string
	if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE school_id=$1 AND code=$2`, schoolID, input.RoleCode).Scan(&roleID); err != nil {
		return PublicUser{}, errors.New("role not found in this school")
	}
	email := strings.ToLower(address.Address)
	_, err = tx.Exec(ctx, `INSERT INTO users(id,school_id,name,email,password_hash,phone) VALUES($1,$2,$3,$4,$5,$6)`, id, schoolID, strings.TrimSpace(input.Name), email, hash, input.Phone)
	if err != nil {
		return PublicUser{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_roles(user_id,role_id) VALUES($1,$2)`, id, roleID); err != nil {
		return PublicUser{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(school_id,user_id,action,resource_type,resource_id,metadata) VALUES($1,$2,'user.created','user',$3,jsonb_build_object('role',$4))`, schoolID, actorID, id, input.RoleCode); err != nil {
		return PublicUser{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PublicUser{}, err
	}
	return PublicUser{ID: id, Name: input.Name, Email: email, Phone: input.Phone, Active: true, Roles: []string{input.RoleCode}}, nil
}

func (s *Store) Disable(ctx context.Context, schoolID, actorID, targetID string) error {
	if actorID == targetID {
		return errors.New("use another privileged administrator to disable this account")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, schoolID+":administrators"); err != nil {
		return err
	}
	var targetAdmin bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_roles ur JOIN roles r ON r.id=ur.role_id WHERE ur.user_id=$1 AND r.school_id=$2 AND r.code='administrator')`, targetID, schoolID).Scan(&targetAdmin)
	if err != nil {
		return err
	}
	if targetAdmin {
		var activeAdmins int
		if err := tx.QueryRow(ctx, `SELECT count(DISTINCT u.id) FROM users u JOIN user_roles ur ON ur.user_id=u.id JOIN roles r ON r.id=ur.role_id WHERE u.school_id=$1 AND u.active=true AND r.code='administrator'`, schoolID).Scan(&activeAdmins); err != nil {
			return err
		}
		if activeAdmins <= 1 {
			return errors.New("cannot disable the final active administrator")
		}
	}
	result, err := tx.Exec(ctx, `UPDATE users SET active=false,disabled_at=now(),updated_at=now() WHERE id=$1 AND school_id=$2 AND active=true`, targetID, schoolID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("active user not found")
	}
	_, _ = tx.Exec(ctx, `UPDATE user_sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, targetID)
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(school_id,user_id,action,resource_type,resource_id) VALUES($1,$2,'user.disabled','user',$3)`, schoolID, actorID, targetID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Reactivate(ctx context.Context, schoolID, actorID, targetID string) error {
	result, err := s.db.Exec(ctx, `WITH changed AS (UPDATE users SET active=true,disabled_at=NULL,updated_at=now() WHERE id=$1 AND school_id=$2 AND active=false RETURNING id) INSERT INTO audit_events(school_id,user_id,action,resource_type,resource_id) SELECT $2,$3,'user.reactivated','user',id FROM changed`, targetID, schoolID, actorID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("disabled user not found")
	}
	return nil
}
