package enrollments

import "time"

type Enrollment struct {
	ID, SchoolID, StudentID, ClassID, Status string
	EnrolledAt                               time.Time
	CompletedAt                              *time.Time
	FinalScoreScaled                         *int
	FinalResult                              *string
	CertificateID                            *string
	CreatedAt, UpdatedAt                     time.Time
}
