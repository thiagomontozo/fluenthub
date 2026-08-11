package levels

type CourseLevel struct {
	ID, CourseID, Name, Code, Description string
	Order                                 int
	Active                                bool
}
