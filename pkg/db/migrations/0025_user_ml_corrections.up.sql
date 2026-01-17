-- +goose Up
-- Migration: Add ML Corrections Table for User Learning Persistence
-- This stores the "delta" between the factory baseline model and user-specific corrections

CREATE TABLE user_ml_corrections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    term TEXT NOT NULL,
    predicted_tag TEXT,
    corrected_tag TEXT NOT NULL,
    model_type TEXT NOT NULL DEFAULT 'TEXT',
    source_file_id UUID REFERENCES user_files (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (user_id, term, model_type)
);

CREATE INDEX idx_user_ml_corrections_user_id ON user_ml_corrections (user_id);

CREATE INDEX idx_user_ml_corrections_term ON user_ml_corrections (term);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_user_ml_corrections_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trigger_user_ml_corrections_updated
    BEFORE UPDATE ON user_ml_corrections
    FOR EACH ROW
    EXECUTE FUNCTION update_user_ml_corrections_timestamp();

COMMENT ON
TABLE user_ml_corrections IS 'Stores user corrections to ML predictions for personalized model learning';

-- +goose Down
DROP TRIGGER IF EXISTS trigger_user_ml_corrections_updated ON user_ml_corrections;

DROP FUNCTION IF EXISTS update_user_ml_corrections_timestamp ();

DROP TABLE IF EXISTS user_ml_corrections;