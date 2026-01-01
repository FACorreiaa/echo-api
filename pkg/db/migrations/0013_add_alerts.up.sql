-- +goose Up
-- Alerts table for pace notifications and insights triggers

CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

-- Alert classification
alert_type VARCHAR(50) NOT NULL, -- 'pace_warning', 'surprise_expense', 'goal_progress', 'subscription_due'
severity VARCHAR(20) NOT NULL DEFAULT 'info', -- 'info', 'warning', 'critical'

-- Alert content
title VARCHAR(255) NOT NULL, message TEXT NOT NULL,

-- Context data (JSON for flexibility)
metadata JSONB,

-- Reference to trigger (optional)
reference_type VARCHAR(50), -- 'transaction', 'category', 'goal', 'subscription'
reference_id UUID,

-- State
is_read BOOLEAN NOT NULL DEFAULT false,
is_dismissed BOOLEAN NOT NULL DEFAULT false,

-- Deduplication: only one alert of same type per day per user
alert_date DATE NOT NULL DEFAULT CURRENT_DATE,
    
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    read_at TIMESTAMP WITH TIME ZONE,
    dismissed_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for common queries
CREATE INDEX idx_alerts_user_unread ON alerts (user_id, is_read)
WHERE
    is_read = false;

CREATE INDEX idx_alerts_user_date ON alerts (user_id, alert_date DESC);

CREATE INDEX idx_alerts_type_date ON alerts (
    user_id,
    alert_type,
    alert_date
);

-- Unique constraint: one alert of each type per user per day
CREATE UNIQUE INDEX idx_alerts_dedup ON alerts (
    user_id,
    alert_type,
    alert_date
);

-- +goose Down
DROP TABLE IF EXISTS alerts;