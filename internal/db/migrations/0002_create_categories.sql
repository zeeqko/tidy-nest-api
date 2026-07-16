-- userId is nullable: NULL means a default (system) category shared by all users.
CREATE TABLE Categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    userId INTEGER REFERENCES Users (id) ON UPDATE CASCADE ON DELETE CASCADE,
    name TEXT NOT NULL,
    icon TEXT,
    reminderOnExpiry INTEGER NOT NULL DEFAULT 0,
    createdAt TEXT NOT NULL,
    updatedAt TEXT NOT NULL
);
