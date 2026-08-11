package audit

import "time"

type Event struct {
	ID, SchoolID         string
	UserID               *string
	Action, ResourceType string
	ResourceID           *string
	Metadata             map[string]any
	IPAddress, UserAgent string
	CreatedAt            time.Time
}
