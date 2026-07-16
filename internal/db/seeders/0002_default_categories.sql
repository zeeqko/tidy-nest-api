-- Default categories shown in the client UI (client/src/data/categories.ts).
-- userId is NULL so they are treated as system defaults for every user.
INSERT INTO Categories (id, userId, name, icon, reminderOnExpiry, createdAt, updatedAt) VALUES
    (1, NULL, 'Food', 'food', 1, datetime('now'), datetime('now')),
    (2, NULL, 'Clothes', 'clothes', 0, datetime('now'), datetime('now')),
    (3, NULL, 'Makeup', 'makeup', 1, datetime('now'), datetime('now')),
    (4, NULL, 'Shoes', 'shoes', 0, datetime('now'), datetime('now')),
    (5, NULL, 'Bags', 'bags', 0, datetime('now'), datetime('now')),
    (6, NULL, 'Books', 'books', 0, datetime('now'), datetime('now'));
