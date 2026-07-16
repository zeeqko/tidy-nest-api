CREATE TABLE Inventories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    userId INTEGER NOT NULL REFERENCES Users (id) ON UPDATE CASCADE ON DELETE CASCADE,
    name TEXT NOT NULL,
    imageURL TEXT,
    expiryDate TEXT,
    opensOn TEXT,
    purchaseDate TEXT,
    subCategoryId INTEGER REFERENCES SubCategories (id) ON UPDATE CASCADE ON DELETE SET NULL,
    tagId INTEGER REFERENCES ItemTags (id) ON UPDATE CASCADE ON DELETE SET NULL,
    quantity INTEGER NOT NULL DEFAULT 1,
    unitPrice REAL,
    storageLocation TEXT,
    createdAt TEXT NOT NULL,
    updatedAt TEXT NOT NULL
);
