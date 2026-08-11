package lessons

import "time"

type Lesson struct {
	ID, SchoolID, ClassID, TeacherID, Title, Description, Status, RecordingStatus string
	ModuleID, LiveSessionID, RecordingStorageKey                                  *string
	ScheduledStart, ScheduledEnd                                                  time.Time
	MaterialsPublished                                                            bool
	CreatedAt, UpdatedAt                                                          time.Time
}
type Material struct {
	ID, LessonID, Title, Description, Type string
	StorageKey, URL                        *string
	ClientFileName, MimeType               string
	Size                                   int64
	CreatedAt                              time.Time
}
