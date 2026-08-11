package users

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thiagomontozo/fluenthub/backend/internal/auth"
	"time"
)

type SessionUser struct {
	ID, SchoolID, Name, Email string
	Roles, Permissions        []string
}
type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }
func (s *Store) Authenticate(ctx context.Context, email, password string) (SessionUser, string, error) {
	var u SessionUser
	var hash string
	var active bool
	err := s.db.QueryRow(ctx, `SELECT id,school_id,name,email,password_hash,active FROM users WHERE lower(email)=lower($1)`, email).Scan(&u.ID, &u.SchoolID, &u.Name, &u.Email, &hash, &active)
	if err != nil || !active || !auth.CheckPassword(hash, password) {
		return SessionUser{}, "", errors.New("invalid credentials")
	}
	raw, digest, err := auth.NewToken()
	if err != nil {
		return SessionUser{}, "", err
	}
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	_, err = s.db.Exec(ctx, `INSERT INTO user_sessions(user_id,token_hash,expires_at) VALUES($1,$2,$3)`, u.ID, digest, expires)
	if err != nil {
		return SessionUser{}, "", err
	}
	_, _ = s.db.Exec(ctx, `UPDATE users SET last_login_at=now() WHERE id=$1`, u.ID)
	return s.withAccess(ctx, u), raw, nil
}
func (s *Store) ByToken(ctx context.Context, raw string) (SessionUser, error) {
	var u SessionUser
	err := s.db.QueryRow(ctx, `SELECT u.id,u.school_id,u.name,u.email FROM user_sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND u.active=true`, auth.Digest(raw)).Scan(&u.ID, &u.SchoolID, &u.Name, &u.Email)
	if err != nil {
		return SessionUser{}, err
	}
	return s.withAccess(ctx, u), nil
}
func (s *Store) Revoke(ctx context.Context, raw string) error {
	_, err := s.db.Exec(ctx, `UPDATE user_sessions SET revoked_at=now() WHERE token_hash=$1`, auth.Digest(raw))
	return err
}
func (s *Store) ChangePassword(ctx context.Context, userID, current, next string) error {
	var hash string
	if err := s.db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1 AND active=true`, userID).Scan(&hash); err != nil {
		return err
	}
	if !auth.CheckPassword(hash, current) {
		return errors.New("current password is invalid")
	}
	newHash, err := auth.HashPassword(next)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$1,updated_at=now() WHERE id=$2`, newHash, userID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE user_sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) withAccess(ctx context.Context, u SessionUser) SessionUser {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT r.code,p.code FROM user_roles ur JOIN roles r ON r.id=ur.role_id LEFT JOIN role_permissions rp ON rp.role_id=r.id LEFT JOIN permissions p ON p.id=rp.permission_id WHERE ur.user_id=$1`, u.ID)
	if err != nil {
		return u
	}
	defer rows.Close()
	seenRole := map[string]bool{}
	seenPerm := map[string]bool{}
	for rows.Next() {
		var role string
		var permission *string
		if rows.Scan(&role, &permission) == nil {
			if !seenRole[role] {
				u.Roles = append(u.Roles, role)
				seenRole[role] = true
			}
			if permission != nil && !seenPerm[*permission] {
				u.Permissions = append(u.Permissions, *permission)
				seenPerm[*permission] = true
			}
		}
	}
	return u
}
func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
