-- +goose Up
-- Migration: 0016_user_plans
-- Description: Add tables for user financial plans (BYOS feature)

-- Plan source type enum
CREATE TYPE plan_source_type AS ENUM ('manual', 'excel', 'template');

-- Plan status enum
CREATE TYPE plan_status AS ENUM ('draft', 'active', 'archived');

-- Widget types for plan items
CREATE TYPE plan_widget_type AS ENUM ('input', 'slider', 'toggle', 'readonly');

-- Field types for plan items
CREATE TYPE plan_field_type AS ENUM ('currency', 'percentage', 'number', 'text');

-- ============================================================================
-- Main user_plans table
-- ============================================================================

CREATE TABLE user_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status plan_status DEFAULT 'draft',
    source_type plan_source_type DEFAULT 'manual',

-- Excel source metadata
source_file_id UUID REFERENCES user_files (id) ON DELETE SET NULL,
excel_sheet_name TEXT,

-- Dynamic configuration (JSONB for flexible schema)
-- Stores: display prefs, formula mappings, i18n labels
config JSONB NOT NULL DEFAULT '{
        "chart_type": "horizontal_bar",
        "show_percentages": true,
        "formula_mappings": {}
    }',

-- Computed summary totals (denormalized for performance)


total_income_minor BIGINT DEFAULT 0,
    total_expenses_minor BIGINT DEFAULT 0,
    currency_code VARCHAR(3) DEFAULT 'EUR',
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Only one active plan per user
CREATE UNIQUE INDEX idx_user_plans_active ON user_plans (user_id)
WHERE
    status = 'active';

-- Fast lookups by user
CREATE INDEX idx_user_plans_user ON user_plans (user_id);

CREATE INDEX idx_user_plans_status ON user_plans (user_id, status);

-- ============================================================================
-- Category groups (high-level groupings like Fundamentals/Fun/Future)
-- ============================================================================

CREATE TABLE plan_category_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES user_plans(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    color VARCHAR(7),  -- Hex color (e.g., #E8A87C)
    target_percent DECIMAL(5, 2) DEFAULT 0,  -- Target allocation %
    sort_order INT DEFAULT 0,

-- i18n labels stored as JSONB


labels JSONB DEFAULT '{}',
    
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_plan_category_groups_plan ON plan_category_groups (plan_id);

-- ============================================================================
-- Categories within groups
-- ============================================================================

CREATE TABLE plan_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES user_plans(id) ON DELETE CASCADE,
    group_id UUID REFERENCES plan_category_groups(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    icon VARCHAR(50),
    color VARCHAR(7),
    sort_order INT DEFAULT 0,

-- i18n labels


labels JSONB DEFAULT '{}',
    
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_plan_categories_plan ON plan_categories (plan_id);

CREATE INDEX idx_plan_categories_group ON plan_categories (group_id);

-- ============================================================================
-- Individual budget line items
-- ============================================================================

CREATE TABLE plan_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES user_plans(id) ON DELETE CASCADE,
    category_id UUID REFERENCES plan_categories(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,

-- Budget amounts (in minor currency units, e.g., cents)
budgeted_minor BIGINT DEFAULT 0, actual_minor BIGINT DEFAULT 0,

-- Excel mapping for imported plans
excel_cell VARCHAR(20), -- e.g., "B10"
formula TEXT, -- Original Excel formula

-- UI configuration
widget_type plan_widget_type DEFAULT 'input',
field_type plan_field_type DEFAULT 'currency',
sort_order INT DEFAULT 0,

-- Value constraints for sliders
min_value BIGINT, max_value BIGINT,

-- i18n labels for multi-language support


labels JSONB DEFAULT '{}',
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_plan_items_plan ON plan_items (plan_id);

CREATE INDEX idx_plan_items_category ON plan_items (category_id);

-- ============================================================================
-- Trigger: Auto-update plan summary totals
-- ============================================================================

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_plan_totals()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE user_plans
    SET 
        total_income_minor = COALESCE((
            SELECT SUM(budgeted_minor) 
            FROM plan_items 
            WHERE plan_id = COALESCE(NEW.plan_id, OLD.plan_id)
            AND budgeted_minor > 0
        ), 0),
        total_expenses_minor = COALESCE((
            SELECT ABS(SUM(budgeted_minor))
            FROM plan_items 
            WHERE plan_id = COALESCE(NEW.plan_id, OLD.plan_id)
            AND budgeted_minor < 0
        ), 0),
        updated_at = NOW()
    WHERE id = COALESCE(NEW.plan_id, OLD.plan_id);
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trigger_update_plan_totals
AFTER INSERT OR UPDATE OR DELETE ON plan_items
FOR EACH ROW EXECUTE FUNCTION update_plan_totals();

-- ============================================================================
-- Trigger: Auto-update updated_at
-- ============================================================================

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_plan_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trigger_user_plans_updated_at
BEFORE UPDATE ON user_plans
FOR EACH ROW EXECUTE FUNCTION update_plan_updated_at();

CREATE TRIGGER trigger_plan_items_updated_at
BEFORE UPDATE ON plan_items
FOR EACH ROW EXECUTE FUNCTION update_plan_updated_at();

-- ============================================================================
-- Comments for documentation
-- ============================================================================

COMMENT ON
TABLE user_plans IS 'User financial plans supporting manual creation and Excel import (BYOS)';

COMMENT ON
TABLE plan_category_groups IS 'High-level groupings like Fundamentals/Fun/Future You';

COMMENT ON
TABLE plan_categories IS 'Spending/income categories within a plan (e.g., Housing, Transport)';

COMMENT ON
TABLE plan_items IS 'Individual budget line items with amounts and Excel cell mappings';

COMMENT ON COLUMN user_plans.config IS 'JSONB config for chart preferences, formulas, and display settings';

COMMENT ON COLUMN plan_items.excel_cell IS 'Excel cell reference for imported plans (e.g., "B10")';

COMMENT ON COLUMN plan_items.labels IS 'i18n labels as JSONB: {"en": "Monthly Rent", "pt": "Aluguel"}';