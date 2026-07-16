-- Sample inventory items mirroring the client UI demo data (client/src/data/items.ts).
INSERT INTO Inventories (id, userId, name, imageURL, expiryDate, opensOn, purchaseDate, subCategoryId, tagId, quantity, unitPrice, storageLocation, createdAt, updatedAt) VALUES
    (1, 1, 'Whole Milk', NULL, '2026-07-15', '2026-07-09', '2026-07-08', 1, 1, 1, 4.50, 'Kitchen → Fridge', datetime('now'), datetime('now')),
    (2, 1, 'Chicken Breast', NULL, '2026-07-12', NULL, '2026-07-10', 2, 2, 1, 6.20, 'Kitchen → Freezer', datetime('now'), datetime('now')),
    (3, 1, 'Denim Jacket', NULL, NULL, NULL, '2025-11-02', 3, 3, 1, 89.00, 'Bedroom → Closet', datetime('now'), datetime('now')),
    (4, 1, 'Cotton Tee', NULL, NULL, NULL, '2026-04-14', 4, 4, 3, 15.00, 'Bedroom → Closet', datetime('now'), datetime('now')),
    (5, 1, 'Rosewood Lipstick', NULL, '2027-02-01', '2026-02-01', '2026-01-15', 5, NULL, 1, 22.00, 'Bedroom → Makeup Table', datetime('now'), datetime('now')),
    (6, 1, 'White Sneakers', NULL, NULL, NULL, '2026-05-03', 6, NULL, 1, 110.00, 'Entryway → Shoe Rack', datetime('now'), datetime('now')),
    (7, 1, 'Leather Tote', NULL, NULL, NULL, '2025-09-12', 7, NULL, 1, 145.00, 'Hallway → Cabinet', datetime('now'), datetime('now')),
    (8, 1, 'The Great Gatsby', NULL, NULL, NULL, '2026-02-20', 8, NULL, 1, 9.99, 'Living Room → Bookshelf', datetime('now'), datetime('now'));
