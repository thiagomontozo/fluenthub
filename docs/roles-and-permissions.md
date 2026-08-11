# Roles and Permissions

| Capability | Administrator | Teacher | Student | Operator |
| --- | --- | --- | --- | --- |
| School and branding | Manage | View | View | View |
| Users and roles | Manage; protect final admin | Assigned scope | Own profile | Administrative fields if granted |
| Courses/classes | Manage | Assigned classes | Enrolled classes | Read/enroll if granted |
| Lessons/live class | Oversight | Create/start/end assigned | Join enrolled | Read operational schedule |
| Attendance | Manage | Assigned classes | Own attendance | No default write |
| Exercises/exams | Oversight | Author/grade assigned | Attempt assigned | No academic writes |
| Academic policy/override | Manage/authorized override | Read; close if granted | View own result | No default access |
| Billing | Oversight | No default access | Own permitted view | Issue/manage if granted |
| Support | Full queue | Own/assigned | Own | Assigned or take queue if granted |
| Certificates/audit | Issue/revoke/read audit | Read eligible scope | Own | Operational read if granted |

Roles map to granular permission codes. Custom roles can be added per school without schema changes. A permission grant answers “may this user attempt the action”; service-level scope checks answer “may this user act on this exact resource.” The frontend hides unavailable commands, but backend decisions are authoritative.
