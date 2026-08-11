# Domain Model

FluentHub partitions entities by business responsibility.

- **Identity:** School, SchoolBranding, User, Role, Permission, UserSession.
- **Academic Structure:** Unit, Course, CourseLevel, CourseModule, ClassGroup, Enrollment.
- **Learning:** Lesson, LessonMaterial, LiveSession, LessonRecording, AttendanceSession, PresenceSegment, RecordingView.
- **Assessment:** separate Exercise and Exam aggregates, attempts, answers, grades, rubrics, skills and revisions.
- **Operations:** Invoice, Payment, Notification, SupportTicket, Assignment, Message and Attachment.
- **Certification:** AcademicPolicy, AcademicResult, AcademicOverride and Certificate.

```mermaid
erDiagram
  SCHOOL ||--o{ USER : owns
  SCHOOL ||--o{ UNIT : contains
  SCHOOL ||--o{ COURSE : offers
  COURSE ||--o{ COURSE_LEVEL : has
  COURSE_LEVEL ||--o{ COURSE_MODULE : has
  UNIT ||--o{ CLASS_GROUP : hosts
  CLASS_GROUP ||--o{ ENROLLMENT : includes
  USER ||--o{ ENROLLMENT : student
  CLASS_GROUP ||--o{ LESSON : schedules
  LESSON ||--o| LIVE_SESSION : opens
  LESSON ||--o{ ATTENDANCE_SESSION : measures
  CLASS_GROUP ||--o{ EXERCISE : practices
  CLASS_GROUP ||--o{ EXAM : assesses
  EXERCISE ||--o{ EXERCISE_ATTEMPT : receives
  EXAM ||--o{ EXAM_ATTEMPT : receives
  ENROLLMENT ||--|| ACADEMIC_RESULT : closes
  ACADEMIC_RESULT ||--o{ ACADEMIC_OVERRIDE : records
  ENROLLMENT ||--o| CERTIFICATE : earns
  USER ||--o{ SUPPORT_TICKET : requests
  USER ||--o{ INVOICE : receives
```

Most principal entities carry `school_id`. Historical academic records avoid cascading deletion. Files carry metadata and opaque storage keys, while binary payloads remain in object storage.
