-- +goose Up
-- User merchant overrides - stores user corrections for merchant names and categories
-- These take priority over default patterns during sanitization

CREATE TABLE user_merchant_overrides (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

-- The pattern to match (raw merchant string or normalized name)
match_pattern TEXT NOT NULL,
match_type TEXT NOT NULL DEFAULT 'exact' CHECK (
    match_type IN ('exact', 'contains', 'regex')
),

-- The override values
merchant_name TEXT NOT NULL, category TEXT, subcategory TEXT,

-- Tracking
match_count INT NOT NULL DEFAULT 0,
last_matched_at TIMESTAMPTZ,
created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

-- Unique constraint per user per pattern
CONSTRAINT unique_user_pattern UNIQUE (user_id, match_pattern) );

-- Index for fast lookup by user
CREATE INDEX idx_user_merchant_overrides_user_id ON user_merchant_overrides (user_id);

-- Index for pattern matching
CREATE INDEX idx_user_merchant_overrides_pattern ON user_merchant_overrides (match_pattern);

COMMENT ON
TABLE user_merchant_overrides IS 'User-defined merchant name and category corrections that override default patterns';

-- +goose Down
DROP TABLE IF EXISTS user_merchant_overrides;