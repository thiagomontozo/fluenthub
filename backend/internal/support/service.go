package support

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }
func (s *Service) Assign(ctx context.Context, schoolID, actorID, ticketID, assigneeID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM support_tickets t JOIN users u ON u.school_id=t.school_id WHERE t.id=$1 AND t.school_id=$2 AND u.id=$3 AND u.active=true)`, ticketID, schoolID, assigneeID).Scan(&valid); err != nil || !valid {
		return errors.New("ticket or assignee is outside this school")
	}
	_, err = tx.Exec(ctx, `UPDATE support_assignments SET unassigned_at=now() WHERE ticket_id=$1 AND unassigned_at IS NULL`, ticketID)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO support_assignments(ticket_id,assigned_to,assigned_by) VALUES($1,$2,$3)`, ticketID, assigneeID, actorID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE support_tickets SET assigned_to_user_id=$1,status='assigned',updated_at=now() WHERE id=$2`, assigneeID, ticketID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(school_id,user_id,action,resource_type,resource_id,metadata) VALUES($1,$2,'ticket.assigned','support_ticket',$3,jsonb_build_object('assignedTo',$4))`, schoolID, actorID, ticketID, assigneeID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) AddMessage(ctx context.Context, schoolID, actorID, ticketID, message string, internal, staff bool) (string, error) {
	if len(strings.TrimSpace(message)) < 1 || len(message) > 10000 {
		return "", errors.New("message must contain 1 to 10000 characters")
	}
	if internal && !staff {
		return "", errors.New("internal messages require support staff permission")
	}
	var allowed bool
	query := `SELECT EXISTS(SELECT 1 FROM support_tickets WHERE id=$1 AND school_id=$2 AND (requester_user_id=$3 OR opened_by_user_id=$3 OR assigned_to_user_id=$3))`
	args := []any{ticketID, schoolID, actorID}
	if staff {
		query = `SELECT EXISTS(SELECT 1 FROM support_tickets WHERE id=$1 AND school_id=$2)`
		args = []any{ticketID, schoolID}
	}
	if err := s.db.QueryRow(ctx, query, args...).Scan(&allowed); err != nil || !allowed {
		return "", errors.New("ticket is outside the actor scope")
	}
	id := uuid.NewString()
	_, err := s.db.Exec(ctx, `INSERT INTO support_messages(id,ticket_id,author_user_id,message,internal) VALUES($1,$2,$3,$4,$5)`, id, ticketID, actorID, strings.TrimSpace(message), internal)
	return id, err
}
