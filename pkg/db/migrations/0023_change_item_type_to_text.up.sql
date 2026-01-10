-- +goose Up
-- Migration: 0023_change_item_type_to_text
-- Description: Change item_type from ENUM to TEXT to allow empty strings per user requirement

-- Drop default value which relies on the enum
ALTER TABLE plan_items ALTER COLUMN item_type DROP DEFAULT;

-- Alter column to TEXT (VARCHAR)
ALTER TABLE plan_items ALTER COLUMN item_type TYPE VARCHAR(50) USING item_type::text;

-- Drop the enum type
DROP TYPE plan_item_type;

-- +goose Down
-- Recreate the enum type
CREATE TYPE plan_item_type AS ENUM ('budget', 'recurring', 'goal', 'income');

-- Convert back to enum, handling potential empty strings or invalid values by mapping default
ALTER TABLE plan_items 
ALTER COLUMN item_type TYPE plan_item_type 
USING (
    CASE 
        WHEN item_type = '' THEN 'budget'::plan_item_type 
        WHEN item_type = 'budget' THEN 'budget'::plan_item_type
        WHEN item_type = 'recurring' THEN 'recurring'::plan_item_type
        WHEN item_type = 'goal' THEN 'goal'::plan_item_type
        WHEN item_type = 'income' THEN 'income'::plan_item_type
        ELSE 'budget'::plan_item_type 
    END
);

-- Restore default
ALTER TABLE plan_items ALTER COLUMN item_type SET DEFAULT 'budget'::plan_item_type;