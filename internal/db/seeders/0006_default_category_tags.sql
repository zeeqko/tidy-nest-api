-- Link the seeded tags to their categories. ON CONFLICT keeps this a no-op on
-- databases where migration 0007 already backfilled the links.
INSERT INTO CategoryTags (categoryId, tagId)
SELECT c.id, t.id FROM Categories c, ItemTags t
WHERE (c.name = 'Food' AND t.name IN ('Fresh', 'Frozen'))
   OR (c.name = 'Clothes' AND t.name IN ('Winter', 'Summer'))
ON CONFLICT DO NOTHING;
