# Grading

All scores use scaled integers (`8.50 = 850`). Exercise and exam attempts keep objective, manual and final components. Objective scoring only trusts server-held correct options; teacher-scored writing/speaking includes feedback and optional rubric criteria.

Grades may be drafts or published. Publication requires all mandatory manual items. Changing a published result requires a non-empty reason and inserts `GradeRevision` before the current grade is updated in the same transaction. The event also produces `grade.revised` audit metadata.

The academic final score applies basis-point weights from the global policy. Passing additionally considers attendance and required work. Result overrides are permission-gated, reasoned and separately auditable.
