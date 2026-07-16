-- Default subcategories used by the client UI demo data (client/src/data/items.ts).
INSERT INTO SubCategories (id, userId, name, icon, categoryId, createdAt, updatedAt) VALUES
    (1, NULL, 'Dairy', 'milk', 1, datetime('now'), datetime('now')),
    (2, NULL, 'Meat', 'beef', 1, datetime('now'), datetime('now')),
    (3, NULL, 'Outerwear', 'shirt', 2, datetime('now'), datetime('now')),
    (4, NULL, 'Tops', 'shirt', 2, datetime('now'), datetime('now')),
    (5, NULL, 'Cosmetics', 'sparkles', 3, datetime('now'), datetime('now')),
    (6, NULL, 'Sneakers', 'footprints', 4, datetime('now'), datetime('now')),
    (7, NULL, 'Handbags', 'shopping-bag', 5, datetime('now'), datetime('now')),
    (8, NULL, 'Fiction', 'book-open', 6, datetime('now'), datetime('now'));
