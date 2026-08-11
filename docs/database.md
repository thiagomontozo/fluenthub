# Database

SQL migrations are ordered, immutable after release and include reversible development down scripts. PostgreSQL is the source of truth; constraints enforce status vocabularies, positive money, unique enrollment pairs, weight totals and score/percentage ranges.

Foreign keys preserve relationships. Destructive cascades are avoided for enrollment, assessment, grade, certificate and audit history. Users are disabled, and classes/courses/exercises/exams are archived when they own history.

Selective indexes cover tenant scope, case-insensitive user email, unit/class status, enrollment student/class, lesson schedule, assessment windows/deadlines, attendance student/lesson, invoice due/status, ticket queue, verification code and audit time.

Money is integer cents. Scores are integers scaled by 100. Weights and attendance use basis points. These representations are never mixed with floating-point storage. Assessment answers are relational; limited question-specific configuration uses JSONB. Exercise and exam tables remain separate.

Object payloads are outside PostgreSQL. Rows store opaque key, safe client filename, detected MIME, size and domain ownership. Audit JSONB is metadata-only and must exclude passwords, tokens, full answers and file contents.
