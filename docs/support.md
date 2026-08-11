# Support

A ticket records `requesterUserId` (who needs help) and `openedByUserId` (who created it), allowing an operator to open a ticket for another user without losing attribution. Categories, priorities and states drive the queue.

Assignments are append-only history with assigner, assignee, start and end. An operator handles tickets assigned to them unless `support.assign` or queue-taking permission allows otherwise. Messages marked internal are projected only to authorized operators/admins; requester views never receive them.

Attachments use generated storage keys, MIME sniffing, size limits and ticket-level authorization. Allowed initial types are PNG, JPEG, WEBP, PDF and optional TXT. Ticket creation, assignment and closure emit audit events; simple updates may also publish SSE notifications.
