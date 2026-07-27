package model

import "time"

// Pointer fields map to nullable columns; a nil userId on Category,
// SubCategory, or ItemTag marks a default (system) record shared by all users.

type User struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	ProfileImageURL *string   `json:"profileImageURL"`
	Currency        string    `json:"currency"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Category struct {
	ID               int64         `json:"id"`
	UserID           *int64        `json:"userId"`
	Name             string        `json:"name"`
	Icon             *string       `json:"icon"`
	Colour           *string       `json:"colour"`
	ReminderOnExpiry bool          `json:"reminderOnExpiry"`
	CreatedAt        time.Time     `json:"createdAt"`
	UpdatedAt        time.Time     `json:"updatedAt"`
	SubCategories    []SubCategory `json:"subCategories"`
	// Tags offered in this category (via CategoryTags), nested so clients
	// need a single request.
	Tags []ItemTag `json:"tags"`
	// Denormalized item stats for list displays.
	ItemCount int      `json:"itemCount"`
	Locations []string `json:"locations"`
}

type SubCategory struct {
	ID         int64     `json:"id"`
	UserID     *int64    `json:"userId"`
	Name       string    `json:"name"`
	Icon       *string   `json:"icon"`
	CategoryID int64     `json:"categoryId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Category   *Category `json:"category,omitempty"`
}

type ItemTag struct {
	ID     int64   `json:"id"`
	UserID *int64  `json:"userId"`
	Name   string  `json:"name"`
	Colour *string `json:"colour"`
	// Categories this tag is offered in, via the CategoryTags junction table.
	CategoryIDs []int64   `json:"categoryIds"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
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
	Quantity        int64        `json:"quantity"`
	UnitPrice       *float64     `json:"unitPrice"`
	StorageLocation *string      `json:"storageLocation"`
	CreatedAt       string       `json:"createdAt"`
	UpdatedAt       string       `json:"updatedAt"`
	SubCategory     *SubCategory `json:"subCategory,omitempty"`
	Tags            []ItemTag    `json:"tags,omitempty"`
}
