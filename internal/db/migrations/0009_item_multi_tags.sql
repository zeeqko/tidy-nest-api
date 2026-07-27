-- Items can carry multiple tags: move the single Inventories.tagId into an
-- InventoryTags junction table, preserving existing assignments.
CREATE TABLE InventoryTags (
    inventoryId BIGINT NOT NULL REFERENCES Inventories (id) ON UPDATE CASCADE ON DELETE CASCADE,
    tagId BIGINT NOT NULL REFERENCES ItemTags (id) ON UPDATE CASCADE ON DELETE CASCADE,
    PRIMARY KEY (inventoryId, tagId)
);

INSERT INTO InventoryTags (inventoryId, tagId)
SELECT id, tagId FROM Inventories WHERE tagId IS NOT NULL;

ALTER TABLE Inventories DROP COLUMN tagId;
