-- Items previously only reached their category indirectly, through
-- subCategoryId -> SubCategories.categoryId. That meant deleting a
-- subcategory (ON DELETE SET NULL on Inventories.subCategoryId) silently
-- orphaned the item from its category too, and deleting a category never
-- touched Inventories at all, since nothing referenced Categories directly.
--
-- categoryId makes the item -> category link its own persisted column, set
-- alongside subCategoryId whenever an item is created/updated (see
-- ensureSubCategory in inventory_service.go). Deleting a subcategory now only
-- clears subCategoryId; deleting a category cascades and removes the item,
-- matching the product intent that a category's items don't outlive it.
ALTER TABLE Inventories
    ADD COLUMN categoryId BIGINT REFERENCES Categories (id) ON UPDATE CASCADE ON DELETE CASCADE;

UPDATE Inventories i
SET categoryId = sc.categoryId
FROM SubCategories sc
WHERE sc.id = i.subCategoryId AND i.categoryId IS NULL;

CREATE INDEX inventories_categoryid_idx ON Inventories (categoryId);
