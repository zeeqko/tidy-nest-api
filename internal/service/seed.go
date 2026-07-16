package service

import "organizing-app-backend/internal/model"

// seedItems provides demo inventory until the database-backed service lands.
// It mirrors the seed data in internal/db/seeders.
func seedItems() []model.InventoryItem {
	return []model.InventoryItem{
		{
			Name:        "Whole Milk",
			Subtitle:    "Dairy · 1 carton",
			Category:    "Food",
			Subcategory: "Dairy",
			Location:    "Fridge",
			Quantity:    1,
			Tag:         "Fresh",
			Status:      "3 days left",
			Notes:       "Organic, top shelf. Check freshness before using in coffee.",
		},
		{
			Name:        "Chicken Breast",
			Subtitle:    "Meat · 500 g",
			Category:    "Food",
			Subcategory: "Meat",
			Location:    "Freezer",
			Quantity:    1,
			Tag:         "Frozen",
			Status:      "Use today",
			Notes:       "Grass-fed, boneless. Move to the fridge the night before cooking.",
		},
		{
			Name:        "Denim Jacket",
			Subtitle:    "Outerwear · 1 pc",
			Category:    "Clothes",
			Subcategory: "Outerwear",
			Location:    "Bedroom Closet",
			Quantity:    1,
			Tag:         "Winter",
			Notes:       "Vintage wash, dry clean only. Pairs well with white sneakers.",
		},
		{
			Name:        "Cotton Tee",
			Subtitle:    "Tops · 3 pcs",
			Category:    "Clothes",
			Subcategory: "Tops",
			Location:    "Bedroom Closet",
			Quantity:    3,
			Tag:         "Summer",
			Notes:       "Machine wash cold, tumble dry low.",
		},
		{
			Name:        "Rosewood Lipstick",
			Subtitle:    "Cosmetics · 1 pc",
			Category:    "Makeup",
			Subcategory: "Cosmetics",
			Location:    "Makeup Table",
			Quantity:    1,
			Notes:       "Matte finish. Sharpen weekly to keep the tip clean.",
		},
		{
			Name:        "White Sneakers",
			Subtitle:    "Sneakers · 1 pair",
			Category:    "Shoes",
			Subcategory: "Sneakers",
			Location:    "Shoe Rack",
			Quantity:    1,
			Notes:       "Wipe clean after wear. Keep the box for storage during off season.",
		},
		{
			Name:        "Leather Tote",
			Subtitle:    "Handbags · 1 pc",
			Category:    "Bags",
			Subcategory: "Handbags",
			Location:    "Hallway Cabinet",
			Quantity:    1,
			Notes:       "Stuff with tissue paper when not in use to keep its shape.",
		},
		{
			Name:        "The Great Gatsby",
			Subtitle:    "Fiction · 1 copy",
			Category:    "Books",
			Subcategory: "Fiction",
			Location:    "Living Room Bookshelf",
			Quantity:    1,
			Notes:       "First edition cover art. Lend only to trusted friends.",
		},
	}
}
