package grading

import "time"

type Revision struct {
	ID, SchoolID, GradeType, GradeID    string
	PreviousScoreScaled, NewScoreScaled int
	Reason, ChangedBy                   string
	CreatedAt                           time.Time
}
type AcademicOverride struct {
	ID, AcademicResultID, PreviousResult, NewResult, Reason, ApprovedBy string
	CreatedAt                                                           time.Time
}
