package domain

import "testing"

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"Çanta Siyah":            "canta-siyah",
		"  Deri  Ceket (Kadın) ": "deri-ceket-kadin",
		"İpek Şal / Ğ Ü Ö":       "ipek-sal-g-u-o",
		"!!!":                    "item",
		"":                       "item",
		"Men's T-Shirt 2024":     "men-s-t-shirt-2024",
	}
	for in, want := range tests {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSearchText(t *testing.T) {
	if got := SearchText("Deri Çanta,", "AUREA"); got != "deri canta aurea" {
		t.Errorf("SearchText = %q", got)
	}
	words := SearchWords("  ÇANTA  deri% ")
	if len(words) != 2 || words[0] != "canta" || words[1] != "deri" {
		t.Errorf("SearchWords = %v", words)
	}
}
