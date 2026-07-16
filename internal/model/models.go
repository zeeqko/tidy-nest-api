package model

// Pointer fields map to nullable columns; a nil userId on Category,
// SubCategory, or ItemTag marks a default (system) record shared by all users.

type User struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	ProfileImageURL *string `json:"profileImageURL"`
	Currency        string  `json:"currency"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type Category struct {
	ID               int64         `json:"id"`
	UserID           *int64        `json:"userId"`
	Name             string        `json:"name"`
	Icon             *string       `json:"icon"`
	ReminderOnExpiry bool          `json:"reminderOnExpiry"`
	CreatedAt        string        `json:"createdAt"`
	UpdatedAt        string        `json:"updatedAt"`
	SubCategories    []SubCategory `json:"subCategories,omitempty"`
}

type SubCategory struct {
	ID         int64     `json:"id"`
	UserID     *int64    `json:"userId"`
	Name       string    `json:"name"`
	Icon       *string   `json:"icon"`
	CategoryID int64     `json:"categoryId"`
	CreatedAt  string    `json:"createdAt"`
	UpdatedAt  string    `json:"updatedAt"`
	Category   *Category `json:"category,omitempty"`
}

type ItemTag struct {
	ID        int64   `json:"id"`
	UserID    *int64  `json:"userId"`
	Name      string  `json:"name"`
	Colour    *string `json:"colour"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}

// Item is a row in the Inventories table.
type Item struct {
	ID              int64        `json:"id"`
	UserID          int64        `json:"userId"`
	Name            string       `json:"name"`
	ImageURL        *string      `json:"imageURL"`
	ExpiryDate      *string      `json:"expiryDate"`
	OpensOn         *string      `json:"opensOn"`
	PurchaseDate    *string      `json:"purchaseDate"`
	SubCategoryID   *int64       `json:"subCategoryId"`
	TagID           *int64       `json:"tagId"`
	Quantity        int64        `json:"quantity"`
	UnitPrice       *float64     `json:"unitPrice"`
	StorageLocation *string      `json:"storageLocation"`
	CreatedAt       string       `json:"createdAt"`
	UpdatedAt       string       `json:"updatedAt"`
	SubCategory     *SubCategory `json:"subCategory,omitempty"`
	Tag             *ItemTag     `json:"tag,omitempty"`
}
