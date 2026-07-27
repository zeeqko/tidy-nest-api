INSERT INTO Users (id, name, profileImageURL, currency, createdAt, updatedAt)
VALUES (1, 'Zee', NULL, 'USD', now(), now());

SELECT setval(pg_get_serial_sequence('users', 'id'), (SELECT MAX(id) FROM Users));
