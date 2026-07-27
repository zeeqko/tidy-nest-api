-- Default tags used by the client UI demo data.
INSERT INTO ItemTags (id, userId, name, colour, createdAt, updatedAt) VALUES
    (1, NULL, 'Fresh', '#B8EFC0', now(), now()),
    (2, NULL, 'Frozen', '#D6ECFF', now(), now()),
    (3, NULL, 'Winter', '#D6ECFF', now(), now()),
    (4, NULL, 'Summer', '#FFE9A8', now(), now());

SELECT setval(pg_get_serial_sequence('itemtags', 'id'), (SELECT MAX(id) FROM ItemTags));
