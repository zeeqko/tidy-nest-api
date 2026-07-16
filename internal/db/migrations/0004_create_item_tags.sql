-- userId is nullable: NULL means a default (system) tag shared by all users.
CREATE TABLE ItemTags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    userId INTEGER REFERENCES Users (id) ON UPDATE CASCADE ON DELETE CASCADE,
    name TEXT NOT NULL,
    colour TEXT,
    createdAt TEXT NOT NULL,
    updatedAt TEXT NOT NULL
);
