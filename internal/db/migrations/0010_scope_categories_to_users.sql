-- Categories/subcategories were global (userId NULL meant a "shared default"
-- visible to every user), which let any authenticated user read, rename, or
-- delete another user's rows (IDOR). This backfills real owners on existing
-- installs, de-duplicates names so the new per-user uniqueness indexes below
-- can be created, then adds them.
--
-- Fresh installs still seed the default rows with userId = NULL (unchanged,
-- see seeders/0002_default_categories.sql and 0003_default_sub_categories.sql,
-- which run after this migration) — seeders/0007_own_default_categories.sql
-- reassigns those afterwards, during the Seed phase.

-- 1. Categories: infer the owner from any Inventories row filed under one of
--    the category's subcategories (subCategoryId -> categoryId), else fall
--    back to the earliest user in the system.
UPDATE Categories c
SET userId = inferred.ownerId, updatedAt = now()
FROM (
    SELECT sc.categoryId AS categoryId, MIN(i.userId) AS ownerId
    FROM Inventories i
    JOIN SubCategories sc ON sc.id = i.subCategoryId
    GROUP BY sc.categoryId
) inferred
WHERE c.id = inferred.categoryId AND c.userId IS NULL;

UPDATE Categories
SET userId = (SELECT MIN(id) FROM Users), updatedAt = now()
WHERE userId IS NULL AND EXISTS (SELECT 1 FROM Users);

-- 2. SubCategories: inherit the (now-backfilled) parent category's owner, so
--    every subcategory stays reachable under its own category's owner.
UPDATE SubCategories sc
SET userId = c.userId, updatedAt = now()
FROM Categories c
WHERE sc.categoryId = c.id AND sc.userId IS NULL;

-- 3. De-duplicate names per user before the unique indexes go on: later rows
--    sharing a case-insensitive name with an earlier one get a suffix built
--    from the row's own id (the primary key, globally unique across the
--    whole table). A reused sequential " (2)", " (3)", ... counter would
--    collide whenever a pre-existing row already happens to be named that
--    (e.g. two rows named "X" plus a pre-existing "X (2)") -- the id-based
--    suffix can't collide that way, since no two rows share an id and no
--    ordinary user-typed name is expected to embed another row's raw id.
WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY userId, lower(name) ORDER BY id) AS rn
    FROM Categories
)
UPDATE Categories c
SET name = c.name || ' (' || c.id || ')', updatedAt = now()
FROM ranked
WHERE c.id = ranked.id AND ranked.rn > 1;

WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY userId, categoryId, lower(name) ORDER BY id) AS rn
    FROM SubCategories
)
UPDATE SubCategories sc
SET name = sc.name || ' (' || sc.id || ')', updatedAt = now()
FROM ranked
WHERE sc.id = ranked.id AND ranked.rn > 1;

-- 4. Case-insensitive uniqueness: category name unique per user, subcategory
--    name unique per (user, category). Rows with userId still NULL (only
--    possible transiently between seeders on a fresh install) are exempt,
--    since Postgres never treats NULL as equal to NULL in a unique index.
CREATE UNIQUE INDEX categories_user_name_unique ON Categories (userId, lower(name));
CREATE UNIQUE INDEX subcategories_user_category_name_unique ON SubCategories (userId, categoryId, lower(name));
