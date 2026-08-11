# Certificates

Eligibility requires a completed enrollment, an approved closed academic result and all policy requirements. Administrators with `certificates.issue` may generate/regenerate. A failed result cannot receive a certificate.

The certificate record contains student/course/level references, workload, completion date, optional final score, human-readable number, random verification code and PDF storage key. The PDF renderer offers classic and modern templates, validated brand colors and a QR link to `/certificate/verify`, while minimizing personal data.

Public verification returns status, name, course, level, completion, workload and number only. Revocation sets actor, time and reason, retains the original record and emits `certificate.revoked`. No email, phone, address or document identifier is public.
