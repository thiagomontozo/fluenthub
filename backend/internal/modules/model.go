package modules

type CourseModule struct {
	ID, LevelID, Name, Description string
	Order, EstimatedHours          int
	Active                         bool
}
