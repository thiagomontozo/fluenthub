# Exams

Exam remains separate from Exercise because formal assessment needs stronger lifecycle, section, timing, publication and answer-disclosure controls. It owns `ExamSection`, exam-specific questions/options, attempts, answers and grades.

The server validates the open/close window and attempt limit, calculates `expiresAt` from duration capped by closing time, and permits incremental answer saves only while the attempt is active. A 15-second technical transport tolerance is documented in code; the browser countdown is informational. A scheduler can transition expired attempts even if JavaScript is closed.

Objective scoring happens server-side. Manual questions block final publication until reviewed. Correct answers remain hidden unless both exam policy permits them and the result is eligible to be shown. Shuffle order must be persisted per attempt when implemented so autosave remains stable.
