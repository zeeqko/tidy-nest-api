package model

import "strings"

const (
	GroupClothes     = "clothes"
	GroupShoes       = "shoes"
	GroupAccessories = "accessories"
	GroupBags        = "bags"
)

// IsValidStylebookGroup reports whether s is one of the four group values.
// The empty string (hidden from the Stylebook) is not a valid group.
func IsValidStylebookGroup(s string) bool {
	switch s {
	case GroupClothes, GroupShoes, GroupAccessories, GroupBags:
		return true
	}
	return false
}

// DefaultStylebookGroup guesses a group from a category name, or returns ""
// when nothing matches. Order matters: "Footwear" must resolve to shoes
// before the "wear" clothes rule is reached.
func DefaultStylebookGroup(name string) string {
	n := strings.ToLower(name)
	has := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(n, s) {
				return true
			}
		}
		return false
	}
	switch {
	case has("shoe", "footwear"):
		return GroupShoes
	case has("accessor"):
		return GroupAccessories
	case has("bag"):
		return GroupBags
	case has("cloth", "top", "bottom", "apparel", "wear"):
		return GroupClothes
	}
	return ""
}
