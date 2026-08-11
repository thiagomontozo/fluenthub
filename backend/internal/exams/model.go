package exams

import "time"

type Exam struct {
	ID, SchoolID, ClassID, TeacherID, Title, Description, Status                string
	ModuleID                                                                    *string
	OpensAt, ClosesAt                                                           *time.Time
	DurationMinutes, MaxScoreScaled, AttemptLimit                               int
	ShuffleQuestions, ShuffleOptions, ShowResultImmediately, ShowCorrectAnswers bool
	CreatedAt, UpdatedAt                                                        time.Time
}
type Section struct {
	ID, ExamID, Title, Description string
	Order, MaxScoreScaled          int
}
type Question struct {
	ID, ExamID, SectionID, Type, Prompt string
	Order, MaxScoreScaled               int
	Required                            bool
	Configuration                       map[string]any
}
type SafeOption struct {
	ID, Label, Value string
	Order            int
}
type Option struct {
	SafeOption
	Correct bool
}
type Attempt struct {
	ID, ExamID, StudentID, Status                             string
	StartedAt, ExpiresAt                                      time.Time
	SubmittedAt                                               *time.Time
	ObjectiveScoreScaled, ManualScoreScaled, FinalScoreScaled *int
}
type Answer struct {
	ID, AttemptID, QuestionID string
	TextAnswer                *string
	SelectedOptionIDs         []string
	SavedAt                   time.Time
}
type Grade struct {
	ID, AttemptID, TeacherID, Feedback, Status                string
	ObjectiveScoreScaled, ManualScoreScaled, FinalScoreScaled int
	GradedAt, PublishedAt                                     *time.Time
}

func NewAttempt(exam Exam, studentID string, now time.Time) (Attempt, error) {
	expires := now.Add(time.Duration(exam.DurationMinutes) * time.Minute)
	if exam.ClosesAt != nil && expires.After(*exam.ClosesAt) {
		expires = *exam.ClosesAt
	}
	return Attempt{ExamID: exam.ID, StudentID: studentID, Status: "in_progress", StartedAt: now, ExpiresAt: expires}, nil
}
func CanSave(attempt Attempt, now time.Time) bool {
	return attempt.Status == "in_progress" && !now.After(attempt.ExpiresAt.Add(15*time.Second))
}
func PublicOptions(options []Option) []SafeOption {
	safe := make([]SafeOption, len(options))
	for i, o := range options {
		safe[i] = o.SafeOption
	}
	return safe
}
