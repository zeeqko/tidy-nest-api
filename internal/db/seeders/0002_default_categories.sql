-- Default categories shown in the client UI (client/src/data/categories.ts).
-- userId is NULL so they are treated as system defaults for every user.
INSERT INTO Categories (id, userId, name, icon, reminderOnExpiry, createdAt, updatedAt) VALUES
    (1, NULL, 'Food', 'food', TRUE, now(), now()),
    (2, NULL, 'Clothes', 'clothes', FALSE, now(), now()),
    (3, NULL, 'Makeup', 'makeup', TRUE, now(), now()),
    (4, NULL, 'Shoes', 'shoes', FALSE, now(), now()),
    (5, NULL, 'Bags', 'bags', FALSE, now(), now()),
    (6, NULL, 'Books', 'books', FALSE, now(), now());

SELECT setval(pg_get_serial_sequence('categories', 'id'), (SELECT MAX(id) FROM Categories));
