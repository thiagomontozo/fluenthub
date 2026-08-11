package notifications

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SendInput struct {
	UserID, Type, Title, Message string
	ResourceType, ResourceID     *string
}
type Service struct {
	db  *pgxpool.Pool
	hub *Hub
}

func NewService(db *pgxpool.Pool, hub *Hub) *Service { return &Service{db: db, hub: hub} }
func (s *Service) Send(ctx context.Context, schoolID, actorID string, input SendInput) (string, error) {
	if input.UserID == "" || strings.TrimSpace(input.Type) == "" || len(strings.TrimSpace(input.Title)) < 2 || len(strings.TrimSpace(input.Message)) < 2 {
		return "", errors.New("recipient, type, title and message are required")
	}
	var owns bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND school_id=$2 AND active=true)`, input.UserID, schoolID).Scan(&owns); err != nil || !owns {
		return "", errors.New("recipient not found in this school")
	}
	id := uuid.NewString()
	_, err := s.db.Exec(ctx, `INSERT INTO notifications(id,school_id,user_id,type,title,message,resource_type,resource_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, schoolID, input.UserID, input.Type, input.Title, input.Message, input.ResourceType, input.ResourceID)
	if err != nil {
		return "", err
	}
	_ = s.hub.Publish(ctx, input.UserID, "notification.created", map[string]string{"id": id, "type": input.Type, "title": input.Title, "message": input.Message})
	return id, nil
}
func (s *Service) MarkRead(ctx context.Context, schoolID, userID, id string) error {
	result, err := s.db.Exec(ctx, `UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE id=$1 AND school_id=$2 AND user_id=$3`, id, schoolID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("notification not found")
	}
	return nil
}
func (s *Service) MarkAllRead(ctx context.Context, schoolID, userID string) error {
	_, err := s.db.Exec(ctx, `UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE school_id=$1 AND user_id=$2 AND read_at IS NULL`, schoolID, userID)
	return err
}
