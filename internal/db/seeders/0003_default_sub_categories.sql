-- Default subcategories used by the client UI demo data.
INSERT INTO SubCategories (id, userId, name, icon, categoryId, createdAt, updatedAt) VALUES
    (1, NULL, 'Dairy', 'milk', 1, now(), now()),
    (2, NULL, 'Meat', 'beef', 1, now(), now()),
    (3, NULL, 'Outerwear', 'shirt', 2, now(), now()),
    (4, NULL, 'Tops', 'shirt', 2, now(), now()),
    (5, NULL, 'Cosmetics', 'sparkles', 3, now(), now()),
    (6, NULL, 'Sneakers', 'footprints', 4, now(), now()),
    (7, NULL, 'Handbags', 'shopping-bag', 5, now(), now()),
    (8, NULL, 'Fiction', 'book-open', 6, now(), now());

SELECT setval(pg_get_serial_sequence('subcategories', 'id'), (SELECT MAX(id) FROM SubCategories));
