package certificates

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type PublicCertificate struct {
	Status, StudentName, CourseName, LevelName, CertificateNumber string
	CompletionDate                                                time.Time
	WorkloadHours                                                 int
	FinalScoreScaled                                              *int
}
type Service struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Service { return &Service{db: db} }
func (s *Service) Verify(ctx context.Context, code string) (PublicCertificate, error) {
	if len(code) < 12 || len(code) > 80 {
		return PublicCertificate{}, errors.New("invalid verification code")
	}
	var c PublicCertificate
	var revoked *time.Time
	err := s.db.QueryRow(ctx, `SELECT CASE WHEN c.revoked_at IS NULL THEN 'valid' ELSE 'revoked' END,u.name,co.name,COALESCE(cl.name,''),c.certificate_number,c.completion_date,c.workload_hours,c.final_score_scaled,c.revoked_at FROM certificates c JOIN users u ON u.id=c.student_id JOIN courses co ON co.id=c.course_id LEFT JOIN course_levels cl ON cl.id=c.level_id WHERE c.verification_code=$1`, code).Scan(&c.Status, &c.StudentName, &c.CourseName, &c.LevelName, &c.CertificateNumber, &c.CompletionDate, &c.WorkloadHours, &c.FinalScoreScaled, &revoked)
	return c, err
}
