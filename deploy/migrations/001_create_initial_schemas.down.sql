DROP TRIGGER IF EXISTS update_contents_updated_at ON contents;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP INDEX IF EXISTS idx_contents_created_at;
DROP INDEX IF EXISTS idx_contents_status;
DROP INDEX IF EXISTS idx_contents_user_id;
DROP TABLE IF EXISTS contents;