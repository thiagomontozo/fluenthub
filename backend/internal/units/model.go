package units

import "time"

type Unit struct {
	ID, SchoolID, Name, Code, Address, City, State, PostalCode, Phone, Email, Timezone string
	Active                                                                             bool
	CreatedAt, UpdatedAt                                                               time.Time
}
