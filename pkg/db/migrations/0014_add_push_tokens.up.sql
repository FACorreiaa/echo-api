-- +goose Up
-- Add push token column to users table for Expo Push notifications

ALTER TABLE users
ADD COLUMN IF NOT EXISTS expo_push_token VARCHAR(255);

-- Create index for finding users with push tokens
CREATE INDEX IF NOT EXISTS idx_users_push_token ON users (expo_push_token)
WHERE
    expo_push_token IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_users_push_token;

ALTER TABLE users DROP COLUMN IF EXISTS expo_push_token;