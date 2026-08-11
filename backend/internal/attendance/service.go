package attendance

import "time"

const HeartbeatInterval = 30 * time.Second

type Status string

const (
	Present Status = "present"
	Partial Status = "partial"
	Absent  Status = "absent"
)

type Session struct {
	ID, SchoolID, LessonID, StudentID          string
	JoinedAt, LastSeenAt                       time.Time
	LeftAt                                     *time.Time
	TotalPresentSeconds, LessonDurationSeconds int64
	AttendanceBasisPoints                      int
	Status                                     Status
}
type PresenceSegment struct {
	ID, SessionID        string
	JoinedAt, LastSeenAt time.Time
	LeftAt               *time.Time
	PresentSeconds       int64
}

func Classify(presentSeconds, durationSeconds int64, minimumBasisPoints int) (int, Status) {
	if durationSeconds <= 0 {
		return 0, Absent
	}
	bp := int(presentSeconds * 10000 / durationSeconds)
	if bp > 10000 {
		bp = 10000
	}
	if bp >= minimumBasisPoints {
		return bp, Present
	}
	if bp >= minimumBasisPoints/2 {
		return bp, Partial
	}
	return bp, Absent
}
