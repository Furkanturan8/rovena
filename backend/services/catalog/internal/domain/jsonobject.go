package domain

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// maxJSONObjectBytes keeps one product from carrying an unbounded blob.
const maxJSONObjectBytes = 32 * 1024

var emptyObject = json.RawMessage(`{}`)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidSlug reports whether s is a URL-safe slug (lowercase letters, digits, single dashes).
func ValidSlug(s string) bool { return len(s) <= 80 && slugPattern.MatchString(s) }

// NormalizeObject returns raw as compact JSON when it is a JSON object; empty input becomes {}.
func NormalizeObject(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return emptyObject, nil
	}
	if len(trimmed) > maxJSONObjectBytes || trimmed[0] != '{' {
		return nil, ErrInvalidJSON
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return nil, ErrInvalidJSON
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, trimmed); err != nil {
		return nil, ErrInvalidJSON
	}
	return json.RawMessage(buf.Bytes()), nil
}

// TranslationTerms collects the searchable texts (name, kind, material) of every language
// in a translations object, so a product can be found by its English name too.
func TranslationTerms(raw json.RawMessage) []string {
	var langs map[string]map[string]json.RawMessage
	if err := json.Unmarshal(raw, &langs); err != nil {
		return nil
	}
	var out []string
	for _, fields := range langs {
		for _, key := range []string{"name", "kind", "material"} {
			var text string
			if v, ok := fields[key]; ok && json.Unmarshal(v, &text) == nil {
				if t := strings.TrimSpace(text); t != "" {
					out = append(out, t)
				}
			}
		}
	}
	return out
}
