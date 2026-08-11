package users

import "time"

type User struct {
	ID, SchoolID, Name, Email, PasswordHash string
	Phone                                   *string
	Active                                  bool
	LastLoginAt, DisabledAt                 *time.Time
	CreatedAt, UpdatedAt                    time.Time
}
