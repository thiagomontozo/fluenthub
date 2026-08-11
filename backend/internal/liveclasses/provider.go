package liveclasses

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"time"
)

type Session struct {
	ID, LessonID, Provider, ProviderSessionID, Status        string
	StartedAt, EndedAt, RecordingStartedAt, RecordingEndedAt *time.Time
}
type JoinInfo struct {
	URL, Token string
	ExpiresAt  time.Time
}
type LiveClassProvider interface {
	Name() string
	CreateSession(context.Context, string) (Session, error)
	StartSession(context.Context, Session) (Session, error)
	EndSession(context.Context, Session) (Session, error)
	GetTeacherJoinInfo(context.Context, Session, string) (JoinInfo, error)
	GetStudentJoinInfo(context.Context, Session, string) (JoinInfo, error)
	StartRecording(context.Context, Session) error
	StopRecording(context.Context, Session) error
}
type MockProvider struct{}

func (MockProvider) Name() string { return "mock" }

func (MockProvider) CreateSession(_ context.Context, lessonID string) (Session, error) {
	id := uuid.NewString()
	return Session{ID: id, LessonID: lessonID, Provider: "mock", ProviderSessionID: "demo-" + id, Status: "created"}, nil
}
func (MockProvider) StartSession(_ context.Context, s Session) (Session, error) {
	now := time.Now().UTC()
	s.StartedAt = &now
	s.Status = "live"
	return s, nil
}
func (MockProvider) EndSession(_ context.Context, s Session) (Session, error) {
	now := time.Now().UTC()
	s.EndedAt = &now
	s.Status = "ended"
	return s, nil
}
func (MockProvider) GetTeacherJoinInfo(_ context.Context, s Session, userID string) (JoinInfo, error) {
	return mockJoin(s, userID, "teacher"), nil
}
func (MockProvider) GetStudentJoinInfo(_ context.Context, s Session, userID string) (JoinInfo, error) {
	return mockJoin(s, userID, "student"), nil
}
func (MockProvider) StartRecording(context.Context, Session) error { return nil }
func (MockProvider) StopRecording(context.Context, Session) error  { return nil }
func mockJoin(s Session, userID, role string) JoinInfo {
	return JoinInfo{URL: fmt.Sprintf("/demo/live/%s?role=%s", s.ProviderSessionID, role), Token: "mock-not-a-real-credential-" + userID, ExpiresAt: time.Now().Add(15 * time.Minute)}
}
