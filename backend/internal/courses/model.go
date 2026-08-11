package courses

import "time"

type Course struct {
	ID, SchoolID, Name, Code, Description, Language string
	Active                                          bool
	CreatedAt, UpdatedAt                            time.Time
}
