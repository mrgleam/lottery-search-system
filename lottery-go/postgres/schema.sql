-- Schema for the lottery ticket store.
-- Idempotent: safe to run on every boot.

CREATE TABLE IF NOT EXISTS tickets (
    id          BIGSERIAL PRIMARY KEY,
    num         INTEGER     NOT NULL CHECK (num BETWEEN 0 AND 999999),
    -- 0 = available, 1 = reserved, 2 = sold. These match lottery.Status.
    status      SMALLINT    NOT NULL DEFAULT 0 CHECK (status IN (0, 1, 2)),
    holder      TEXT,
    lease_until TIMESTAMPTZ,
    version     INTEGER     NOT NULL DEFAULT 0
);

-- The claim path. Partial on "not sold" rather than "available" because an
-- expired reservation is claimable again and must stay reachable through this
-- index. Excluding sold rows is what keeps it small as inventory depletes:
-- once a ticket is sold it leaves the index permanently.
CREATE INDEX IF NOT EXISTS tickets_claimable_num
    ON tickets (num)
    WHERE status <> 2;

-- The reaper's index. Tiny, because it only ever covers live reservations.
CREATE INDEX IF NOT EXISTS tickets_expiring
    ON tickets (lease_until)
    WHERE status = 1;

-- Reporting only. Never used by the claim path, which must not depend on
-- anything that scans.
CREATE INDEX IF NOT EXISTS tickets_holder
    ON tickets (holder)
    WHERE holder IS NOT NULL;
