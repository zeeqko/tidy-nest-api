-- Serves the inventory list query (WHERE userId = $1 ORDER BY createdAt, id)
-- so Postgres can walk the index in order instead of sorting every user's
-- rows on each request (see PLAN.md T16). Plain CREATE INDEX, not
-- CONCURRENTLY: the migration runner wraps each file in a transaction.
CREATE INDEX IF NOT EXISTS inventories_userid_createdat_idx ON Inventories (userId, createdAt);
