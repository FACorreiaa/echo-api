-- Migration: Add ML Corrections Table for User Learning Persistence
-- This stores the "delta" between the factory baseline model and user-specific corrections

CREATE TABLE user_ml_corrections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

-- The term from the Excel spreadsheet (e.g., "Netflix", "Lidl")
term TEXT NOT NULL,

-- What the ML model originally predicted
predicted_tag TEXT,

-- What the user corrected it to (B, R, S, IN, D)
corrected_tag TEXT NOT NULL,

-- Model type: TEXT (category name prediction) or STRUCTURE (group/item detection)
model_type TEXT NOT NULL DEFAULT 'TEXT',

-- Metadata
source_file_id UUID REFERENCES user_files (id) ON DELETE SET NULL,
created_at TIMESTAMPTZ DEFAULT NOW(),
updated_at TIMESTAMPTZ DEFAULT NOW(),

-- Ensure we don't store duplicate corrections for the same term/user/model
UNIQUE(user_id, term, model_type) );

-- Index for fast lookup by user
CREATE INDEX idx_user_ml_corrections_user_id ON user_ml_corrections (user_id);

-- Index for analytics on most-corrected terms
CREATE INDEX idx_user_ml_corrections_term ON user_ml_corrections (term);

-- Trigger to update updated_at
CREATE OR REPLACE FUNCTION update_user_ml_corrections_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_user_ml_corrections_updated
    BEFORE UPDATE ON user_ml_corrections
    FOR EACH ROW
    EXECUTE FUNCTION update_user_ml_corrections_timestamp();

-- Add comment for documentation
COMMENT ON
TABLE user_ml_corrections IS 'Stores user corrections to ML predictions for personalized model learning';

COMMENT ON COLUMN user_ml_corrections.term IS 'The lowercase text from Excel that was corrected';

COMMENT ON COLUMN user_ml_corrections.predicted_tag IS 'Original ML prediction (B=Budget, R=Recurring, S=Savings, IN=Income, D=Debt)';

COMMENT ON COLUMN user_ml_corrections.corrected_tag IS 'User-corrected tag';

COMMENT ON COLUMN user_ml_corrections.model_type IS 'Which ML model: TEXT for category prediction, STRUCTURE for group/item detection';