-- +goose Up
-- +goose StatementBegin

-- Store computed quality insights for each import job
CREATE TABLE import_job_insights (
    import_job_id UUID PRIMARY KEY REFERENCES import_jobs(id) ON DELETE CASCADE,
    institution_name TEXT,

-- Quality metrics (0.0 - 1.0 scale)
categorization_rate DECIMAL(5, 4) DEFAULT 0,
date_quality_score DECIMAL(5, 4) DEFAULT 1,
amount_quality_score DECIMAL(5, 4) DEFAULT 1,

-- Date range of imported transactions
earliest_date TIMESTAMPTZ, latest_date TIMESTAMPTZ,

-- Money totals (in minor units)
total_income_minor BIGINT DEFAULT 0,
total_expenses_minor BIGINT DEFAULT 0,
currency_code CHAR(3),

-- Duplicates skipped during import
duplicates_skipped INT DEFAULT 0,

-- Issues detected as JSON array
issues_json JSONB DEFAULT '[]'::jsonb,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT import_job_insights_currency_code_chk CHECK (currency_code IS NULL OR currency_code ~ '^[A-Z]{3}$')
);

CREATE TRIGGER trigger_set_import_job_insights_updated_at
BEFORE UPDATE ON import_job_insights
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Materialized view for data source health (refreshed periodically or on-demand)
-- Aggregates transaction data by institution and source type
CREATE MATERIALIZED VIEW data_source_health AS
SELECT 
    user_id,
    COALESCE(institution_name, 'Unknown') as institution_name,
    source as source_type,
    COUNT(*) as transaction_count,
    MIN(posted_at) as first_transaction,
    MAX(posted_at) as last_transaction,
    MAX(created_at) as last_import,
    ROUND(
        COUNT(*) FILTER (WHERE category_id IS NOT NULL)::DECIMAL / 
        NULLIF(COUNT(*), 0), 4
    ) as categorization_rate,
    COUNT(*) FILTER (WHERE category_id IS NULL) as uncategorized_count
FROM transactions
GROUP BY user_id, COALESCE(institution_name, 'Unknown'), source;

-- Unique index for efficient lookups and concurrent refresh
CREATE UNIQUE INDEX idx_data_source_health_unique ON data_source_health (
    user_id,
    institution_name,
    source_type
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP MATERIALIZED VIEW IF EXISTS data_source_health;

DROP TABLE IF EXISTS import_job_insights;
-- +goose StatementEnd