package assessment

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Service { return &Service{db: db} }

type AnswerInput struct {
	QuestionID        string
	TextAnswer        *string
	SelectedOptionIDs []string
}
type GradeInput struct {
	ScoreScaled    int
	Feedback       string
	Publish        bool
	RevisionReason string
}

func (s *Service) StartExercise(ctx context.Context, schoolID, studentID, exerciseID string) (string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var limit *int
	var available *time.Time
	var due *time.Time
	var status string
	err = tx.QueryRow(ctx, `SELECT x.attempt_limit,x.available_from,x.due_at,x.status FROM exercises x JOIN enrollments e ON e.class_id=x.class_id AND e.student_id=$1 AND e.status='active' WHERE x.id=$2 AND x.school_id=$3 FOR UPDATE`, studentID, exerciseID, schoolID).Scan(&limit, &available, &due, &status)
	if err != nil {
		return "", errors.New("exercise is not available to this student")
	}
	now := time.Now().UTC()
	if status != "published" || (available != nil && now.Before(*available)) || (due != nil && now.After(*due)) {
		return "", errors.New("exercise is outside its availability window")
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM exercise_attempts WHERE exercise_id=$1 AND student_id=$2`, exerciseID, studentID).Scan(&count); err != nil {
		return "", err
	}
	if limit != nil && count >= *limit {
		return "", errors.New("exercise attempt limit reached")
	}
	id := uuid.NewString()
	if _, err := tx.Exec(ctx, `INSERT INTO exercise_attempts(id,exercise_id,student_id,status) VALUES($1,$2,$3,'in_progress')`, id, exerciseID, studentID); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

func (s *Service) SaveExerciseAnswer(ctx context.Context, schoolID, studentID, attemptID string, input AnswerInput) error {
	sort.Strings(input.SelectedOptionIDs)
	result, err := s.db.Exec(ctx, `INSERT INTO exercise_answers(attempt_id,question_id,text_answer,selected_option_ids) SELECT a.id,q.id,$4,$5 FROM exercise_attempts a JOIN exercises x ON x.id=a.exercise_id JOIN exercise_questions q ON q.exercise_id=x.id WHERE a.id=$1 AND a.student_id=$2 AND x.school_id=$3 AND a.status='in_progress' AND q.id=$6 ON CONFLICT(attempt_id,question_id) DO UPDATE SET text_answer=excluded.text_answer,selected_option_ids=excluded.selected_option_ids,updated_at=now()`, attemptID, studentID, schoolID, input.TextAnswer, input.SelectedOptionIDs, input.QuestionID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("attempt or question is outside the student scope")
	}
	return nil
}

func (s *Service) SubmitExercise(ctx context.Context, schoolID, studentID, attemptID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var objective int
	var manual bool
	err = tx.QueryRow(ctx, `SELECT COALESCE(sum(CASE WHEN q.type IN ('multiple_choice','true_false') AND COALESCE(a.selected_option_ids,'{}')=(SELECT COALESCE(array_agg(o.id ORDER BY o.id),'{}') FROM exercise_question_options o WHERE o.question_id=q.id AND o.correct) THEN q.max_score_scaled ELSE 0 END),0),bool_or(q.type NOT IN ('multiple_choice','true_false')) FROM exercise_attempts t JOIN exercises x ON x.id=t.exercise_id JOIN exercise_questions q ON q.exercise_id=x.id LEFT JOIN exercise_answers a ON a.attempt_id=t.id AND a.question_id=q.id WHERE t.id=$1 AND t.student_id=$2 AND x.school_id=$3 AND t.status='in_progress' GROUP BY t.id`, attemptID, studentID, schoolID).Scan(&objective, &manual)
	if err != nil {
		return errors.New("active attempt not found")
	}
	state := "graded"
	if manual {
		state = "awaiting_review"
	}
	_, err = tx.Exec(ctx, `UPDATE exercise_attempts SET submitted_at=now(),status=$1,objective_score_scaled=$2,final_score_scaled=CASE WHEN $3=false THEN $2 ELSE NULL END WHERE id=$4`, state, objective, manual, attemptID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) StartExam(ctx context.Context, schoolID, studentID, examID string) (string, time.Time, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	defer tx.Rollback(ctx)
	var duration, limit int
	var opens, closes *time.Time
	var status string
	err = tx.QueryRow(ctx, `SELECT x.duration_minutes,x.attempt_limit,x.opens_at,x.closes_at,x.status FROM exams x JOIN enrollments e ON e.class_id=x.class_id AND e.student_id=$1 AND e.status='active' WHERE x.id=$2 AND x.school_id=$3 FOR UPDATE`, studentID, examID, schoolID).Scan(&duration, &limit, &opens, &closes, &status)
	if err != nil {
		return "", time.Time{}, errors.New("exam is not available to this student")
	}
	now := time.Now().UTC()
	if status != "open" || (opens != nil && now.Before(*opens)) || (closes != nil && now.After(*closes)) {
		return "", time.Time{}, errors.New("exam is outside its availability window")
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM exam_attempts WHERE exam_id=$1 AND student_id=$2`, examID, studentID).Scan(&count); err != nil {
		return "", time.Time{}, err
	}
	if count >= limit {
		return "", time.Time{}, errors.New("exam attempt limit reached")
	}
	expires := now.Add(time.Duration(duration) * time.Minute)
	if closes != nil && expires.After(*closes) {
		expires = *closes
	}
	id := uuid.NewString()
	if _, err := tx.Exec(ctx, `INSERT INTO exam_attempts(id,exam_id,student_id,expires_at,status) VALUES($1,$2,$3,$4,'in_progress')`, id, examID, studentID, expires); err != nil {
		return "", time.Time{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", time.Time{}, err
	}
	return id, expires, nil
}

func (s *Service) SaveExamAnswer(ctx context.Context, schoolID, studentID, attemptID string, input AnswerInput) error {
	sort.Strings(input.SelectedOptionIDs)
	result, err := s.db.Exec(ctx, `INSERT INTO exam_answers(attempt_id,question_id,text_answer,selected_option_ids) SELECT a.id,q.id,$4,$5 FROM exam_attempts a JOIN exams x ON x.id=a.exam_id JOIN exam_questions q ON q.exam_id=x.id WHERE a.id=$1 AND a.student_id=$2 AND x.school_id=$3 AND a.status='in_progress' AND now()<=a.expires_at+interval '15 seconds' AND q.id=$6 ON CONFLICT(attempt_id,question_id) DO UPDATE SET text_answer=excluded.text_answer,selected_option_ids=excluded.selected_option_ids,saved_at=now()`, attemptID, studentID, schoolID, input.TextAnswer, input.SelectedOptionIDs, input.QuestionID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("attempt expired or question is outside the student scope")
	}
	return nil
}

func (s *Service) SubmitExam(ctx context.Context, schoolID, studentID, attemptID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var objective int
	var manual bool
	var expired bool
	err = tx.QueryRow(ctx, `SELECT COALESCE(sum(CASE WHEN q.type IN ('multiple_choice','true_false') AND COALESCE(a.selected_option_ids,'{}')=(SELECT COALESCE(array_agg(o.id ORDER BY o.id),'{}') FROM exam_question_options o WHERE o.question_id=q.id AND o.correct) THEN q.max_score_scaled ELSE 0 END),0),bool_or(q.type NOT IN ('multiple_choice','true_false')),now()>t.expires_at+interval '15 seconds' FROM exam_attempts t JOIN exams x ON x.id=t.exam_id JOIN exam_questions q ON q.exam_id=x.id LEFT JOIN exam_answers a ON a.attempt_id=t.id AND a.question_id=q.id WHERE t.id=$1 AND t.student_id=$2 AND x.school_id=$3 AND t.status='in_progress' GROUP BY t.id`, attemptID, studentID, schoolID).Scan(&objective, &manual, &expired)
	if err != nil {
		return errors.New("active exam attempt not found")
	}
	state := "graded"
	if manual {
		state = "awaiting_review"
	}
	if expired {
		state = "expired"
	}
	_, err = tx.Exec(ctx, `UPDATE exam_attempts SET submitted_at=now(),status=$1,objective_score_scaled=$2,final_score_scaled=CASE WHEN $3=false THEN $2 ELSE NULL END WHERE id=$4`, state, objective, manual, attemptID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) GradeExercise(ctx context.Context, schoolID, teacherID, attemptID string, input GradeInput) error {
	return s.grade(ctx, "exercise", schoolID, teacherID, attemptID, input)
}
func (s *Service) GradeExam(ctx context.Context, schoolID, teacherID, attemptID string, input GradeInput) error {
	return s.grade(ctx, "exam", schoolID, teacherID, attemptID, input)
}
func (s *Service) grade(ctx, kind, schoolID, teacherID, attemptID string, input GradeInput) error {
	if input.ScoreScaled < 0 {
		return errors.New("score cannot be negative")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	gradeTable, attemptTable, ownerJoin := "exercise_grades", "exercise_attempts", `JOIN exercises x ON x.id=a.exercise_id`
	if kind == "exam" {
		gradeTable, attemptTable, ownerJoin = "exam_grades", "exam_attempts", `JOIN exams x ON x.id=a.exam_id`
	}
	var maxScore int
	if err := tx.QueryRow(ctx, `SELECT x.max_score_scaled FROM `+attemptTable+` a `+ownerJoin+` WHERE a.id=$1 AND x.school_id=$2 AND x.teacher_id=$3`, attemptID, schoolID, teacherID).Scan(&maxScore); err != nil {
		return errors.New("attempt is outside the teacher scope")
	}
	if kind == "exercise" && input.ScoreScaled > maxScore {
		return errors.New("score exceeds exercise maximum")
	}
	var gradeID string
	var previous *int
	var previousStatus *string
	lookup := `SELECT id,final_score_scaled,status FROM ` + gradeTable + ` WHERE attempt_id=$1 FOR UPDATE`
	if kind == "exercise" {
		lookup = `SELECT id,score_scaled,status FROM ` + gradeTable + ` WHERE attempt_id=$1 FOR UPDATE`
	}
	err = tx.QueryRow(ctx, lookup, attemptID).Scan(&gradeID, &previous, &previousStatus)
	if err != nil {
		gradeID = uuid.NewString()
	}
	if previousStatus != nil && *previousStatus == "published" && len(input.RevisionReason) < 5 {
		return errors.New("published grade revision requires a reason")
	}
	status := "draft"
	var publishedAt *time.Time
	if input.Publish {
		status = "published"
		now := time.Now().UTC()
		publishedAt = &now
	}
	revisionScore := input.ScoreScaled
	if kind == "exercise" {
		_, err = tx.Exec(ctx, `INSERT INTO exercise_grades(id,attempt_id,teacher_id,score_scaled,feedback,status,graded_at,published_at) VALUES($1,$2,$3,$4,$5,$6,now(),$7) ON CONFLICT(attempt_id) DO UPDATE SET score_scaled=excluded.score_scaled,feedback=excluded.feedback,status=excluded.status,graded_at=now(),published_at=excluded.published_at`, gradeID, attemptID, teacherID, input.ScoreScaled, input.Feedback, status, publishedAt)
	} else {
		var objective int
		if err = tx.QueryRow(ctx, `SELECT COALESCE(objective_score_scaled,0) FROM exam_attempts WHERE id=$1`, attemptID).Scan(&objective); err == nil {
			revisionScore = objective + input.ScoreScaled
			if revisionScore > maxScore {
				return errors.New("score exceeds exam maximum")
			}
			_, err = tx.Exec(ctx, `INSERT INTO exam_grades(id,attempt_id,teacher_id,objective_score_scaled,manual_score_scaled,final_score_scaled,feedback,status,graded_at,published_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,now(),$9) ON CONFLICT(attempt_id) DO UPDATE SET manual_score_scaled=excluded.manual_score_scaled,final_score_scaled=excluded.final_score_scaled,feedback=excluded.feedback,status=excluded.status,graded_at=now(),published_at=excluded.published_at`, gradeID, attemptID, teacherID, objective, input.ScoreScaled, revisionScore, input.Feedback, status, publishedAt)
		}
	}
	if err != nil {
		return err
	}
	if previous != nil && *previous != revisionScore {
		_, err = tx.Exec(ctx, `INSERT INTO grade_revisions(school_id,grade_type,grade_id,previous_score_scaled,new_score_scaled,reason,changed_by) VALUES($1,$2,$3,$4,$5,$6,$7)`, schoolID, kind, gradeID, *previous, revisionScore, input.RevisionReason, teacherID)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
