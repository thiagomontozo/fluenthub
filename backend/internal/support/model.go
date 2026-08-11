package support

import "time"

type Ticket struct {
	ID, SchoolID, TicketNumber, RequesterUserID, OpenedByUserID, Category, Subject, Description, Priority, Status string
	AssignedToUserID                                                                                              *string
	CreatedAt, UpdatedAt                                                                                          time.Time
	ResolvedAt, ClosedAt                                                                                          *time.Time
}
type Message struct {
	ID, TicketID, AuthorUserID, Message string
	Internal                            bool
	CreatedAt                           time.Time
}
type Assignment struct {
	ID, TicketID, AssignedTo, AssignedBy string
	AssignedAt                           time.Time
	UnassignedAt                         *time.Time
}
type Attachment struct {
	ID, TicketID, MessageID, StorageKey, ClientFileName, MimeType string
	Size                                                          int64
	CreatedAt                                                     time.Time
}
