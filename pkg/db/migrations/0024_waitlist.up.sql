-- +goose Up
-- Migration: 0024_waitlist
-- Description: Waitlist table for pre-launch signups from echo-os.com landing page

CREATE TABLE waitlist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    email TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (
        status IN (
            'pending',
            'invited',
            'joined'
        )
    ),
    invite_code TEXT UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    invited_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ
);

-- Ensure email uniqueness (case-insensitive)
CREATE UNIQUE INDEX idx_waitlist_email_unique ON waitlist (LOWER(email));

-- Indexes for common queries
CREATE INDEX idx_waitlist_status ON waitlist (status);

CREATE INDEX idx_waitlist_created_at ON waitlist (created_at DESC);

CREATE INDEX idx_waitlist_invite_code ON waitlist (invite_code)
WHERE
    invite_code IS NOT NULL;

-- Documentation comments
COMMENT ON
TABLE waitlist IS 'Pre-launch waitlist signups from echo-os.com landing page';

COMMENT ON COLUMN waitlist.status IS 'pending = waiting, invited = sent early access, joined = created account';

COMMENT ON COLUMN waitlist.invite_code IS 'Unique 8-char alphanumeric code for early access activation';

-- +goose Down
DROP TABLE IF EXISTS waitlist;