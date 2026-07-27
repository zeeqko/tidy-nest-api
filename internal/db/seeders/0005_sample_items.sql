-- Sample inventory items mirroring the client UI demo data.
INSERT INTO Inventories (id, userId, name, imageURL, expiryDate, opensOn, purchaseDate, subCategoryId, quantity, unitPrice, storageLocation, notes, createdAt, updatedAt) VALUES
    (1, 1, 'Whole Milk', NULL, '2026-07-15', '2026-07-09', '2026-07-08', 1, 1, 4.50, 'Kitchen → Fridge', 'Organic, top shelf. Check freshness before using in coffee.', now(), now()),
    (2, 1, 'Chicken Breast', NULL, '2026-07-12', NULL, '2026-07-10', 2, 1, 6.20, 'Kitchen → Freezer', 'Grass-fed, boneless. Move to the fridge the night before cooking.', now(), now()),
    (3, 1, 'Denim Jacket', NULL, NULL, NULL, '2025-11-02', 3, 1, 89.00, 'Bedroom → Closet', 'Vintage wash, dry clean only. Pairs well with white sneakers.', now(), now()),
    (4, 1, 'Cotton Tee', NULL, NULL, NULL, '2026-04-14', 4, 3, 15.00, 'Bedroom → Closet', 'Machine wash cold, tumble dry low.', now(), now()),
    (5, 1, 'Rosewood Lipstick', NULL, '2027-02-01', '2026-02-01', '2026-01-15', 5, 1, 22.00, 'Bedroom → Makeup Table', 'Matte finish. Sharpen weekly to keep the tip clean.', now(), now()),
    (6, 1, 'White Sneakers', NULL, NULL, NULL, '2026-05-03', 6, 1, 110.00, 'Entryway → Shoe Rack', 'Wipe clean after wear. Keep the box for storage during off season.', now(), now()),
    (7, 1, 'Leather Tote', NULL, NULL, NULL, '2025-09-12', 7, 1, 145.00, 'Hallway → Cabinet', 'Stuff with tissue paper when not in use to keep its shape.', now(), now()),
    (8, 1, 'The Great Gatsby', NULL, NULL, NULL, '2026-02-20', 8, 1, 9.99, 'Living Room → Bookshelf', 'First edition cover art. Lend only to trusted friends.', now(), now());

INSERT INTO InventoryTags (inventoryId, tagId) VALUES
    (1, 1),
    (2, 2),
    (3, 3),
    (4, 4);

SELECT setval(pg_get_serial_sequence('inventories', 'id'), (SELECT MAX(id) FROM Inventories));
