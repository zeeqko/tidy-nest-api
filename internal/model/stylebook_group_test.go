package model

import "testing"

func TestDefaultStylebookGroup(t *testing.T) {
	cases := map[string]string{
		"Clothes": "clothes", "Tops": "clothes", "Bottoms": "clothes",
		"Shoes": "shoes", "Footwear": "shoes", "FOOTWEAR": "shoes",
		"Accessories": "accessories", "Handbags": "bags", "Bags": "bags",
		"Food": "", "Books": "", "Makeup": "",
	}
	for name, want := range cases {
		if got := DefaultStylebookGroup(name); got != want {
			t.Errorf("DefaultStylebookGroup(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestIsValidStylebookGroup(t *testing.T) {
	for _, s := range []string{"clothes", "shoes", "accessories", "bags"} {
		if !IsValidStylebookGroup(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range []string{"", "hats"} {
		if IsValidStylebookGroup(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}
