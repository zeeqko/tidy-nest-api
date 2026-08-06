package model

import "time"

// LookItem is a single inventory item placed on a Look's canvas, carrying
// the transform it was arranged with. Name/ImageURL/CutoutURL are
// response-only, resolved by joining Inventories — ignored on write (only
// ItemID is read).
type LookItem struct {
	ItemID    string  `json:"itemId"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Rotation  float64 `json:"rotation"`
	ZIndex    int     `json:"zIndex"`
	Name      string  `json:"name,omitempty"`
	ImageURL  string  `json:"imageURL,omitempty"`
	CutoutURL string  `json:"cutoutURL,omitempty"`
}

// Look is a saved outfit: a named, optionally occasion-tagged arrangement of
// inventory items placed freeform on a canvas.
type Look struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Occasion  string     `json:"occasion"`
	Items     []LookItem `json:"items"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}
