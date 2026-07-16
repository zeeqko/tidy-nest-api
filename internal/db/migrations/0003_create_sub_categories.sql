-- userId is nullable: NULL means a default (system) subcategory shared by all users.
CREATE TABLE SubCategories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    userId INTEGER REFERENCES Users (id) ON UPDATE CASCADE ON DELETE CASCADE,
    name TEXT NOT NULL,
    icon TEXT,
    categoryId INTEGER NOT NULL REFERENCES Categories (id) ON UPDATE CASCADE ON DELETE CASCADE,
    createdAt TEXT NOT NULL,
    updatedAt TEXT NOT NULL
);
