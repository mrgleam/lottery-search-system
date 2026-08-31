-- Optional migration for ShardedStore. Apply ONLY after measuring that lock
-- contention, not inventory depletion or pool saturation, is your bottleneck.

ALTER TABLE tickets ADD COLUMN IF NOT EXISTS shard SMALLINT NOT NULL DEFAULT 0;

-- Spread existing rows across 16 shards. On a large table do this in batches
-- to avoid one enormous transaction.
UPDATE tickets SET shard = (id % 16)::smallint WHERE shard = 0;

-- The sharded claim path needs shard first: the query filters on an exact
-- shard, then on a set of numbers.
CREATE INDEX IF NOT EXISTS tickets_shard_claimable
    ON tickets (shard, num)
    WHERE status <> 2;

-- The unsharded index is now redundant for claims. Drop it only once you have
-- confirmed nothing else uses it -- check pg_stat_user_indexes.idx_scan first.
-- DROP INDEX IF EXISTS tickets_claimable_num;
