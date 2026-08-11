package classes

import "time"

type Status string

const (
	Planned   Status = "planned"
	Open      Status = "open"
	Active    Status = "active"
	Completed Status = "completed"
	Cancelled Status = "cancelled"
	Archived  Status = "archived"
)

type ClassGroup struct {
	ID, SchoolID, UnitID, CourseID, LevelID, Name, Code string
	TeacherID                                           *string
	StartDate, EndDate                                  time.Time
	Capacity                                            int
	ScheduleDescription                                 string
	Status                                              Status
	CreatedAt, UpdatedAt                                time.Time
}
