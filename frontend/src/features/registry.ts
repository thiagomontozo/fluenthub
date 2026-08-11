export const featureAreas = {
  auth: ['login', 'logout', 'me', 'change-password'],
  admin: ['units', 'courses', 'classes', 'users', 'academic-policy', 'audit'],
  teacher: ['lessons', 'live-class', 'attendance', 'exercise-grading', 'exam-grading'],
  student: ['classes', 'recordings', 'exercises', 'exams', 'grades', 'performance'],
  operator: ['students', 'enrollments', 'billing', 'notifications', 'support'],
  learning: ['lessons', 'live-class', 'attendance', 'recording-views'],
  assessment: ['exercises', 'exams', 'grading', 'language-skills'],
  operations: ['billing', 'notifications', 'support', 'certificates', 'branding'],
} as const
