-- Optional catalog seed. It contains no personal or operational data.
-- Run only after creating a school through the setup wizard.
CREATE TABLE IF NOT EXISTS schema_metadata (key text PRIMARY KEY,value text NOT NULL);
INSERT INTO schema_metadata(key,value) VALUES('version','0.1.0') ON CONFLICT(key) DO UPDATE SET value=excluded.value;
