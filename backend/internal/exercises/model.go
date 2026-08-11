package exercises

import "time"

type QuestionType string

const (
	MultipleChoice QuestionType = "multiple_choice"
	TrueFalse      QuestionType = "true_false"
	ShortText      QuestionType = "short_text"
	LongText       QuestionType = "long_text"
	FillBlank      QuestionType = "fill_blank"
	Listening      QuestionType = "listening"
	Speaking       QuestionType = "speaking"
	Matching       QuestionType = "matching"
)

type Exercise struct {
	ID, SchoolID, ClassID, TeacherID, Title, Description, Status string
	LessonID, ModuleID                                           *string
	AvailableFrom, DueAt                                         *time.Time
	MaxScoreScaled                                               int
	AttemptLimit                                                 *int
	CreatedAt, UpdatedAt                                         time.Time
}
type Question struct {
	ID, ExerciseID        string
	Type                  QuestionType
	Prompt                string
	Order, MaxScoreScaled int
	Required              bool
	Configuration         map[string]any
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
	ID, ExerciseID, StudentID, Status                         string
	StartedAt                                                 time.Time
	SubmittedAt                                               *time.Time
	ObjectiveScoreScaled, ManualScoreScaled, FinalScoreScaled *int
}
type Answer struct {
	ID, AttemptID, QuestionID string
	TextAnswer                *string
	SelectedOptionIDs         []string
	CreatedAt, UpdatedAt      time.Time
}
type AudioSubmission struct {
	ID, AttemptID, QuestionID, StorageKey, MimeType string
	DurationSeconds                                 int
	Size                                            int64
	CreatedAt                                       time.Time
}
type Grade struct {
	ID, AttemptID, TeacherID, Feedback, Status string
	ScoreScaled                                int
	GradedAt, PublishedAt                      *time.Time
}
type GradingCriterion struct {
	ID, SchoolID, Name, Description string
	MaxScoreScaled, Order           int
	Active                          bool
}
type CriterionScore struct {
	ID, CriterionID, GradeType, GradeID string
	ScoreScaled                         int
	Feedback                            string
}

func PublicOptions(options []Option) []SafeOption {
	safe := make([]SafeOption, len(options))
	for i, o := range options {
		safe[i] = o.SafeOption
	}
	return safe
}
