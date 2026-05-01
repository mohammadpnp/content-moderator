-- deploy/docker/init-db.sql
-- This script runs once when PostgreSQL container is first created

-- Create enum types
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'content_type') THEN
        CREATE TYPE content_type AS ENUM ('text', 'image');
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'content_status') THEN
        CREATE TYPE content_status AS ENUM ('pending', 'approved', 'rejected');
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'notification_type') THEN
        CREATE TYPE notification_type AS ENUM ('approved', 'rejected');
    END IF;
END $$;

-- Tables will be created by migrations in Phase 1
-- This is just for reference

COMMENT ON DATABASE content_moderator IS 'Content Moderation System Database';