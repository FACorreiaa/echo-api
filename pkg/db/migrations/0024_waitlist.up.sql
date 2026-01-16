-- Waitlist table for pre-launch signups
-- Stores email signups from the landing page with invite tracking

CREATE TABLE waitlist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'invited', 'joined')),
    invite_code TEXT UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    invited_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ,

-- Ensure email uniqueness (case-insensitive)
CONSTRAINT waitlist_email_unique UNIQUE (LOWER(email)) );

-- Indexes for common queries
CREATE INDEX idx_waitlist_email ON waitlist (LOWER(email));

CREATE INDEX idx_waitlist_status ON waitlist (status);

CREATE INDEX idx_waitlist_created_at ON waitlist (created_at DESC);

CREATE INDEX idx_waitlist_invite_code ON waitlist (invite_code)
WHERE
    invite_code IS NOT NULL;

-- Add comment for documentation
COMMENT ON
TABLE waitlist IS 'Pre-launch waitlist signups from echo-os.com landing page';

COMMENT ON COLUMN waitlist.status IS 'pending = waiting, invited = sent early access, joined = created account';

COMMENT ON COLUMN waitlist.invite_code IS 'Unique 8-char alphanumeric code for early access activation';