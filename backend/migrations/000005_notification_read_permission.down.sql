DELETE FROM role_permissions WHERE permission_id=(SELECT id FROM permissions WHERE code='notifications.read');
DELETE FROM permissions WHERE code='notifications.read';
