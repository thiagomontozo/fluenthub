package liveclasses

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db       *pgxpool.Pool
	provider LiveClassProvider
}

func NewService(db *pgxpool.Pool, provider LiveClassProvider) *Service {
	return &Service{db: db, provider: provider}
}

func (service *Service) Create(ctx context.Context, schoolID, actorID, lessonID string, unrestricted bool) (Session, error) {
	var teacherID string
	if err := service.db.QueryRow(ctx, `SELECT teacher_id FROM lessons WHERE id=$1 AND school_id=$2 AND status IN ('draft','scheduled')`, lessonID, schoolID).Scan(&teacherID); err != nil {
		return Session{}, errors.New("eligible lesson not found")
	}
	if teacherID != actorID && !unrestricted {
		return Session{}, errors.New("lesson is outside teacher scope")
	}
	session, err := service.provider.CreateSession(ctx, lessonID)
	if err != nil {
		return Session{}, err
	}
	session.ID = uuid.NewString()
	tx, err := service.db.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO live_sessions(id,lesson_id,provider,provider_session_id,status) VALUES($1,$2,$3,$4,$5)`, session.ID, lessonID, session.Provider, session.ProviderSessionID, session.Status)
	if err != nil {
		return Session{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE lessons SET live_session_id=$1,updated_at=now() WHERE id=$2`, session.ID, lessonID); err != nil {
		return Session{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(school_id,user_id,action,resource_type,resource_id,metadata) VALUES($1,$2,'live_session.created','live_session',$3,jsonb_build_object('provider',$4))`, schoolID, actorID, session.ID, session.Provider); err != nil {
		return Session{}, err
	}
	return session, tx.Commit(ctx)
}

func (service *Service) Start(ctx context.Context, schoolID, actorID, sessionID string, unrestricted bool) (Session, error) {
	session, teacherID, err := service.load(ctx, schoolID, sessionID)
	if err != nil {
		return Session{}, err
	}
	if teacherID != actorID && !unrestricted {
		return Session{}, errors.New("live session is outside teacher scope")
	}
	session, err = service.provider.StartSession(ctx, session)
	if err != nil {
		return Session{}, err
	}
	_, err = service.db.Exec(ctx, `UPDATE live_sessions SET status='live',started_at=$1 WHERE id=$2; UPDATE lessons SET status='live',updated_at=now() WHERE id=$3`, session.StartedAt, session.ID, session.LessonID)
	return session, err
}

func (service *Service) End(ctx context.Context, schoolID, actorID, sessionID string, unrestricted bool) (Session, error) {
	session, teacherID, err := service.load(ctx, schoolID, sessionID)
	if err != nil {
		return Session{}, err
	}
	if teacherID != actorID && !unrestricted {
		return Session{}, errors.New("live session is outside teacher scope")
	}
	session, err = service.provider.EndSession(ctx, session)
	if err != nil {
		return Session{}, err
	}
	_, err = service.db.Exec(ctx, `UPDATE live_sessions SET status='ended',ended_at=$1 WHERE id=$2; UPDATE lessons SET status='completed',updated_at=now() WHERE id=$3`, session.EndedAt, session.ID, session.LessonID)
	return session, err
}

func (service *Service) Join(ctx context.Context, schoolID, userID, sessionID string, unrestricted bool) (JoinInfo, error) {
	session, teacherID, err := service.load(ctx, schoolID, sessionID)
	if err != nil {
		return JoinInfo{}, err
	}
	if session.Status != "created" && session.Status != "live" {
		return JoinInfo{}, errors.New("live session is not joinable")
	}
	if teacherID == userID || unrestricted {
		return service.provider.GetTeacherJoinInfo(ctx, session, userID)
	}
	var enrolled bool
	err = service.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM enrollments e JOIN lessons l ON l.class_id=e.class_id WHERE l.id=$1 AND e.student_id=$2 AND e.school_id=$3 AND e.status='active')`, session.LessonID, userID, schoolID).Scan(&enrolled)
	if err != nil || !enrolled {
		return JoinInfo{}, errors.New("student is not actively enrolled in this class")
	}
	return service.provider.GetStudentJoinInfo(ctx, session, userID)
}

func (service *Service) JoinLesson(ctx context.Context, schoolID, userID, lessonID string, unrestricted bool) (JoinInfo, error) {
	var sessionID string
	if err := service.db.QueryRow(ctx, `SELECT s.id FROM live_sessions s JOIN lessons l ON l.id=s.lesson_id WHERE l.id=$1 AND l.school_id=$2 AND s.status IN ('created','live')`, lessonID, schoolID).Scan(&sessionID); err != nil {
		return JoinInfo{}, errors.New("joinable live session not found")
	}
	return service.Join(ctx, schoolID, userID, sessionID, unrestricted)
}

func (service *Service) Recording(ctx context.Context, schoolID, actorID, sessionID string, start, unrestricted bool) error {
	session, teacherID, err := service.load(ctx, schoolID, sessionID)
	if err != nil {
		return err
	}
	if teacherID != actorID && !unrestricted {
		return errors.New("live session is outside teacher scope")
	}
	if session.Status != "live" {
		return errors.New("recording requires a live session")
	}
	if start {
		if err := service.provider.StartRecording(ctx, session); err != nil {
			return err
		}
		_, err = service.db.Exec(ctx, `UPDATE live_sessions SET recording_started_at=now() WHERE id=$1`, session.ID)
		return err
	}
	if err := service.provider.StopRecording(ctx, session); err != nil {
		return err
	}
	_, err = service.db.Exec(ctx, `UPDATE live_sessions SET recording_ended_at=now() WHERE id=$1`, session.ID)
	return err
}

func (service *Service) load(ctx context.Context, schoolID, sessionID string) (Session, string, error) {
	var session Session
	var teacherID string
	err := service.db.QueryRow(ctx, `SELECT s.id,s.lesson_id,s.provider,s.provider_session_id,s.status,s.started_at,s.ended_at,s.recording_started_at,s.recording_ended_at,l.teacher_id FROM live_sessions s JOIN lessons l ON l.id=s.lesson_id WHERE s.id=$1 AND l.school_id=$2`, sessionID, schoolID).Scan(&session.ID, &session.LessonID, &session.Provider, &session.ProviderSessionID, &session.Status, &session.StartedAt, &session.EndedAt, &session.RecordingStartedAt, &session.RecordingEndedAt, &teacherID)
	if err != nil {
		return Session{}, "", errors.New("live session not found")
	}
	if session.Provider != service.provider.Name() {
		return Session{}, "", errors.New("live session belongs to a different configured provider")
	}
	return session, teacherID, nil
}
