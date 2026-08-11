package roles

type Role struct {
	ID, SchoolID, Name, Code, Description string
	System                                bool
	Permissions                           []string
}
