package http_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
)

func (e *env) lookup(key, body string) result {
	req := httptest.NewRequest("POST", "/api/v1/internal/variants/lookup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("X-Internal-Key", key)
	}
	return e.send(req)
}

func TestVariantLookup_RequiresInternalKey(t *testing.T) {
	e := newEnv(t)
	p := e.seedProduct("Linen Shirt", domain.StatusActive, active("M", 1500))
	body := `{"variant_ids":["` + p.Variants[0].ID.String() + `"]}`

	for name, key := range map[string]string{"missing": "", "wrong": "x" + testInternalKey} {
		if res := e.lookup(key, body); res.Status != 401 {
			t.Errorf("%s key: status %d, want 401", name, res.Status)
		}
	}
	// A valid admin token is not the internal key.
	req := httptest.NewRequest("POST", "/api/v1/internal/variants/lookup", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	req.Header.Set("Content-Type", "application/json")
	if res := e.send(req); res.Status != 401 {
		t.Errorf("admin token: status %d, want 401", res.Status)
	}
}

func TestVariantLookup_ReturnsPriceNameAndSellability(t *testing.T) {
	e := newEnv(t)
	live := e.seedProduct("Linen Shirt", domain.StatusActive, active("M", 1500), inactive("L", 1600))
	draft := e.seedProduct("Draft Hat", domain.StatusDraft, active("S", 900))
	ids := []string{live.Variants[0].ID.String(), live.Variants[1].ID.String(), draft.Variants[0].ID.String(), "00000000-0000-4000-8000-000000000000"}
	res := e.lookup(testInternalKey, `{"variant_ids":["`+strings.Join(ids, `","`)+`"]}`)
	if res.Status != 200 {
		t.Fatalf("status %d: %s", res.Status, res.Raw)
	}
	items := res.Body["data"].([]any)
	if len(items) != 3 {
		t.Fatalf("unknown ids must be left out, got %d items", len(items))
	}
	byID := map[string]map[string]any{}
	for _, it := range items {
		m := it.(map[string]any)
		byID[m["variant_id"].(string)] = m
	}
	if m := byID[ids[0]]; m["sellable"] != true || m["price_cents"].(float64) != 1500 || m["product_name"] != "Linen Shirt" {
		t.Errorf("active variant: %v", m)
	}
	if byID[ids[1]]["sellable"] != false {
		t.Error("inactive variant must not be sellable")
	}
	if byID[ids[2]]["sellable"] != false {
		t.Error("variant of a draft product must not be sellable")
	}
}

func TestVariantLookup_Validation(t *testing.T) {
	e := newEnv(t)
	for name, body := range map[string]string{
		"empty list": `{"variant_ids":[]}`,
		"not a uuid": `{"variant_ids":["nope"]}`,
		"no body":    `{}`,
	} {
		if res := e.lookup(testInternalKey, body); res.Status != 422 && res.Status != 400 {
			t.Errorf("%s: status %d", name, res.Status)
		}
	}
}
