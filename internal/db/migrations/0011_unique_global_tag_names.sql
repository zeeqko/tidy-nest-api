-- attachDefaultTag (default_categories.go) previously relied on a
-- transaction-scoped advisory lock to avoid concurrent signups racing to
-- create the same global (userId IS NULL) ItemTags name. That still let
-- duplicates in from hand-created rows during earlier tickets' manual UI
-- testing (e.g. "Cozy" ids 21/22, several "DupTag <timestamp>" pairs), and
-- was never as airtight as a real constraint. This dedupes existing
-- duplicates, repoints any CategoryTags/InventoryTags links from a
-- to-be-deleted duplicate onto the surviving row, then adds the unique
-- index the new INSERT ... ON CONFLICT upsert in attachDefaultTag targets.

-- 1. For each case-insensitive name among NULL-owner ItemTags, the
--    lowest-id row survives; every other row with that name is a duplicate
--    to remove.
WITH dupes AS (
    SELECT id, MIN(id) OVER (PARTITION BY lower(name)) AS survivorId
    FROM ItemTags
    WHERE userId IS NULL
),
losers AS (
    SELECT id, survivorId FROM dupes WHERE id <> survivorId
)
-- 2. Repoint CategoryTags links from a losing duplicate's id onto the
--    survivor's id first, so no category silently loses a tag once the
--    duplicate is deleted. ON CONFLICT DO NOTHING handles a category that
--    already links both the duplicate and the survivor.
INSERT INTO CategoryTags (categoryId, tagId, createdAt)
SELECT ct.categoryId, losers.survivorId, ct.createdAt
FROM CategoryTags ct
JOIN losers ON losers.id = ct.tagId
ON CONFLICT DO NOTHING;

-- 2b. Repoint InventoryTags the same way and for the same reason as step 2:
--     InventoryTags.tagId REFERENCES ItemTags (id) ON DELETE CASCADE (see
--     migrations/0009_item_multi_tags.sql), so deleting a losing duplicate
--     in step 3 below would otherwise silently cascade-delete any item's
--     link to it instead of erroring — the item would silently lose its tag
--     with no trace afterward. ON CONFLICT DO NOTHING handles an item that
--     already carries both the duplicate and the survivor tag.
WITH dupes AS (
    SELECT id, MIN(id) OVER (PARTITION BY lower(name)) AS survivorId
    FROM ItemTags
    WHERE userId IS NULL
),
losers AS (
    SELECT id, survivorId FROM dupes WHERE id <> survivorId
)
INSERT INTO InventoryTags (inventoryId, tagId)
SELECT it.inventoryId, losers.survivorId
FROM InventoryTags it
JOIN losers ON losers.id = it.tagId
ON CONFLICT DO NOTHING;

-- 3. Delete the losing duplicate ItemTags rows. Any CategoryTags/
--    InventoryTags rows still pointing at them (now redundant with the
--    repointed links from steps 2/2b, or skipped by ON CONFLICT DO NOTHING
--    above) cascade-delete via their tagId FKs.
WITH dupes AS (
    SELECT id, MIN(id) OVER (PARTITION BY lower(name)) AS survivorId
    FROM ItemTags
    WHERE userId IS NULL
)
DELETE FROM ItemTags
WHERE id IN (SELECT id FROM dupes WHERE id <> survivorId);

-- 4. Enforce case-insensitive uniqueness among global tag names going
-- forward. attachDefaultTag's upsert (default_categories.go) targets this
-- exact index as its ON CONFLICT clause.
CREATE UNIQUE INDEX itemtags_name_unique ON ItemTags (lower(name)) WHERE userId IS NULL;
