# Exercises

Exercise is the practice-oriented assessment aggregate. It owns questions, options, availability, deadline, maximum score, attempt limit and draft/published/closed/archived lifecycle.

Supported question types are multiple choice, true/false, short text, long text, fill blank, listening, speaking and simple matching. Specific settings use constrained configuration; central business rules remain typed code and relational fields. Correct option flags are removed from the student projection.

Objective questions can be scored server-side. Short text is not auto-corrected without an explicit authored rule. Writing and speaking enter `awaiting_review`; the teacher saves a draft score and feedback before publication. Speaking audio uses opaque storage keys and optional configurable rubric criteria. Attempt limit and time window are enforced when starting and submitting.
