package domain

import "strings"

var turkishReplacer = strings.NewReplacer(
	"ç", "c", "Ç", "c", "ğ", "g", "Ğ", "g", "ı", "i", "İ", "i",
	"ö", "o", "Ö", "o", "ş", "s", "Ş", "s", "ü", "u", "Ü", "u",
)

// Slugify turns a display name into a URL-safe slug ("Çanta Siyah" -> "canta-siyah").
func Slugify(s string) string {
	s = strings.ToLower(turkishReplacer.Replace(s))
	var b strings.Builder
	lastDash := true
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		return "item"
	}
	if len(out) > 80 {
		out = strings.TrimRight(out[:80], "-")
	}
	return out
}

// SearchText normalises text for accent-insensitive search: lowercase, Turkish letters
// folded to ASCII, everything else collapsed to single spaces ("Çanta, Deri" -> "canta deri").
func SearchText(parts ...string) string {
	return strings.Join(searchWords(strings.Join(parts, " ")), " ")
}

// SearchWords splits a user query into normalised words.
func SearchWords(query string) []string { return searchWords(query) }

func searchWords(s string) []string {
	s = strings.ToLower(turkishReplacer.Replace(s))
	return strings.FieldsFunc(s, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
}
