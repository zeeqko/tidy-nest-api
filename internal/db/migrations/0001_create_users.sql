CREATE TABLE Users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    profileImageURL TEXT,
    currency TEXT NOT NULL DEFAULT 'USD',
    createdAt TEXT NOT NULL,
    updatedAt TEXT NOT NULL
);
