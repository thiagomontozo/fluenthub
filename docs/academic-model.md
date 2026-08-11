# Academic Model

`Course` defines a language offering. Levels are ordered but not forced to CEFR. Modules divide a level into teachable units. A `ClassGroup` combines unit, course, level, schedule, capacity and optionally a teacher. An `Enrollment` connects one student to one class and is unique for that pair.

`AcademicPolicy` belongs to a school. Assessment weights use basis points and total 10,000; scores use scale 100. With exercise average 8,500, exam average 7,200 and weights 3,000/7,000, the rounded final score is 7,590 (75.90 on a 0–100 presentation scale when configured that way).

`AcademicResult` also checks minimum live attendance and mandatory work. The result remains `in_progress` until calculable, may become `pending_review`, and closes as `approved` or `failed`. A change after closure is not a direct update: `academic.override` permission, a reason, an `AcademicOverride` row and audit event are mandatory.

Progress combines completed modules, completed/available lessons and mandatory activities; deployments should document the selected normalized weights. Video watched alone never equals course completion.
