package schools

import "time"

type School struct {
	ID, LegalName, DisplayName, Slug, Email, Phone, Website, Timezone, Locale string
	Active                                                                    bool
	CreatedAt, UpdatedAt                                                      time.Time
}
