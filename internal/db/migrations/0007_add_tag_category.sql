-- Junction table linking tags to the categories they are offered in.
-- Removing a tag from a category only deletes the link; the tag itself (and
-- its use on items) survives in other categories.
CREATE TABLE CategoryTags (
    categoryId BIGINT NOT NULL REFERENCES Categories (id) ON UPDATE CASCADE ON DELETE CASCADE,
    tagId BIGINT NOT NULL REFERENCES ItemTags (id) ON UPDATE CASCADE ON DELETE CASCADE,
    createdAt TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (categoryId, tagId)
);

-- Backfill the seeded tags into their natural categories on existing
-- databases. The filtered cross join yields no rows if the category is gone.
INSERT INTO CategoryTags (categoryId, tagId)
SELECT c.id, t.id FROM Categories c, ItemTags t
WHERE c.name = 'Food' AND t.name IN ('Fresh', 'Frozen')
ON CONFLICT DO NOTHING;

INSERT INTO CategoryTags (categoryId, tagId)
SELECT c.id, t.id FROM Categories c, ItemTags t
WHERE c.name = 'Clothes' AND t.name IN ('Winter', 'Summer')
ON CONFLICT DO NOTHING;
