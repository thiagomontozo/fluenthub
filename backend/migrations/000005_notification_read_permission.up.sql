INSERT INTO permissions(code,description) VALUES('notifications.read','Read own in-app notifications') ON CONFLICT(code) DO NOTHING;
