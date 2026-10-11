package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/catalog/internal/application"
	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	transport "github.com/Furkanturan8/rovena/services/catalog/internal/transport/http"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	testSecret      = "test-secret-test-secret-test-secret-1234"
	testInternalKey = "internal-key-internal-key-internal-key-1"
)

type env struct {
	t     *testing.T
	app   *fiber.App
	store *store

	readyErr error
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{t: t, store: newStore()}
	app := transport.NewApp(transport.Options{
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		AllowedOrigins: "https://shop.example",
		TrustedProxies: []string{"127.0.0.1"},
		Verifier:       auth.NewVerifier([]byte(testSecret)),
		InternalAPIKey: testInternalKey,
		Readiness: map[string]httpx.ReadinessCheck{
			"postgres": func(context.Context) error { return e.readyErr },
		},
	}, transport.Services{
		Categories: application.NewCategoryService(categoryRepo{e.store}),
		Products:   application.NewProductService(productRepo{e.store}, variantRepo{e.store}),
	})
	e.app = app
	return e
}

func token(t *testing.T, role string, secret string, expiresIn time.Duration) string {
	t.Helper()
	claims := auth.Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    auth.Issuer,
			Subject:   uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func adminToken(t *testing.T) string { return token(t, auth.RoleAdmin, testSecret, time.Hour) }

type result struct {
	Status int
	Header http.Header
	Raw    []byte
	Body   map[string]any
}

func (r result) data() map[string]any {
	d, _ := r.Body["data"].(map[string]any)
	return d
}

func (r result) fields() map[string]any {
	f, _ := r.Body["fields"].(map[string]any)
	return f
}

func (r result) message() string {
	m, _ := r.Body["message"].(string)
	return m
}

// do sends a request; body may be nil, a string (sent verbatim) or a value to JSON-encode.
func (e *env) do(method, path, bearer string, body any) result {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return e.send(req)
}

func (e *env) send(req *http.Request) result {
	e.t.Helper()
	resp, err := e.app.Test(req, -1)
	if err != nil {
		e.t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	res := result{Status: resp.StatusCode, Header: resp.Header, Raw: raw}
	_ = json.Unmarshal(raw, &res.Body)
	return res
}

func (e *env) admin(method, path string, body any) result {
	return e.do(method, path, adminToken(e.t), body)
}

// seedProduct inserts a product straight into the in-memory store.
func (e *env) seedProduct(name string, status domain.ProductStatus, variants ...domain.Variant) *domain.Product {
	e.t.Helper()
	p := &domain.Product{
		ID: uuid.New(), CategoryID: uuid.New(), Name: name, Slug: domain.Slugify(name),
		Status: status, Brand: "Rovena", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Images: []domain.Image{{URL: "https://img.test/" + domain.Slugify(name) + ".jpg", Position: 0}},
	}
	for i, v := range variants {
		v.ID, v.ProductID, v.Currency = uuid.New(), p.ID, domain.DefaultCurrency
		v.SKU = fmt.Sprintf("%s-%d", strings.ToUpper(domain.Slugify(name)), i)
		p.Variants = append(p.Variants, v)
	}
	e.store.products[p.ID] = p
	return p
}

func active(size string, price int64) domain.Variant {
	return domain.Variant{Size: size, PriceCents: price, Active: true}
}

func inactive(size string, price int64) domain.Variant {
	return domain.Variant{Size: size, PriceCents: price, Active: false}
}

// ---------------------------------------------------------------- public API

func TestPublicList_ParsesQueryIntoFilter(t *testing.T) {
	e := newEnv(t)
	res := e.do("GET", "/api/v1/products?q=%C3%A7anta&category=giyim&size=M&color=Siyah&min_price=100&max_price=900&sort=price_asc&page=2&page_size=10", "", nil)
	if res.Status != 200 {
		t.Fatalf("status = %d: %s", res.Status, res.Raw)
	}
	f := e.store.lastFilter
	if f.Query != "çanta" || f.CategorySlug != "giyim" || f.Size != "M" || f.Color != "Siyah" {
		t.Errorf("text filters = %+v", f)
	}
	if f.MinPrice == nil || *f.MinPrice != 100 || f.MaxPrice == nil || *f.MaxPrice != 900 {
		t.Errorf("price filters = %v..%v", f.MinPrice, f.MaxPrice)
	}
	if f.Sort != domain.SortPriceAsc || f.Page != 2 || f.PageSize != 10 {
		t.Errorf("sort/paging = %s page %d size %d", f.Sort, f.Page, f.PageSize)
	}
	if f.Status == nil || *f.Status != domain.StatusActive {
		t.Errorf("public listing must force status=active, got %v", f.Status)
	}
}

func TestPublicList_DefaultsAndClamping(t *testing.T) {
	tests := []struct {
		query    string
		page, sz int
	}{
		{"", 1, 20},
		{"?page=-5&page_size=0", 1, 1},
		{"?page=abc&page_size=abc", 1, 20},
		{"?page_size=100000", 1, 100},
	}
	for _, tt := range tests {
		e := newEnv(t)
		if res := e.do("GET", "/api/v1/products"+tt.query, "", nil); res.Status != 200 {
			t.Fatalf("%q: status %d", tt.query, res.Status)
		}
		if f := e.store.lastFilter; f.Page != tt.page || f.PageSize != tt.sz || f.Sort != domain.SortNewest {
			t.Errorf("%q: page %d size %d sort %s, want %d/%d/newest", tt.query, f.Page, f.PageSize, f.Sort, tt.page, tt.sz)
		}
	}
}

func TestPublicList_RejectsBadParameters(t *testing.T) {
	bad := []string{
		"sort=evil",
		"sort=price_asc;DROP+TABLE+products",
		"min_price=abc",
		"min_price=-1",
		"max_price=1.5",
		"min_price=500&max_price=100",
		"q=" + strings.Repeat("a", 101),
	}
	for _, q := range bad {
		e := newEnv(t)
		res := e.do("GET", "/api/v1/products?"+q, "", nil)
		if res.Status != 400 {
			t.Errorf("%q: status = %d, want 400", q, res.Status)
		}
		if res.Body["success"] != false || res.message() == "" {
			t.Errorf("%q: body = %s", q, res.Raw)
		}
		if e.store.listCalls != 0 {
			t.Errorf("%q: repository was called despite invalid input", q)
		}
	}
}

func TestPublicList_ResponseShapeAndHiding(t *testing.T) {
	e := newEnv(t)
	e.seedProduct("Deri Ceket", domain.StatusActive, active("M", 459900), active("L", 479900))
	e.seedProduct("Taslak", domain.StatusDraft, active("M", 100))
	e.seedProduct("Eski Model", domain.StatusArchived, active("M", 100))

	res := e.do("GET", "/api/v1/products", "", nil)
	if res.Status != 200 || res.Body["success"] != true {
		t.Fatalf("status %d: %s", res.Status, res.Raw)
	}
	d := res.data()
	if d["total"] != float64(1) || d["page"] != float64(1) || d["page_size"] != float64(20) {
		t.Errorf("page meta = %v", d)
	}
	items := d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %d, want only the active product", len(items))
	}
	item := items[0].(map[string]any)
	if item["name"] != "Deri Ceket" || item["slug"] != "deri-ceket" || item["price_from_cents"] != float64(459900) ||
		item["currency"] != "TRY" || item["thumbnail_url"] != "https://img.test/deri-ceket.jpg" {
		t.Errorf("item = %v", item)
	}
	if _, leaked := item["variants"]; leaked {
		t.Error("list items must not carry variants")
	}
}

func TestPublicList_EmptyIsEmptyArrayNotNull(t *testing.T) {
	e := newEnv(t)
	res := e.do("GET", "/api/v1/products", "", nil)
	if !strings.Contains(string(res.Raw), `"items":[]`) {
		t.Errorf("body = %s, want items to be []", res.Raw)
	}
}

func TestPublicDetail(t *testing.T) {
	e := newEnv(t)
	live := e.seedProduct("Deri Ceket", domain.StatusActive, active("M", 459900), inactive("S", 100))
	draft := e.seedProduct("Taslak Urun", domain.StatusDraft, active("M", 100))

	for _, ref := range []string{live.Slug, live.ID.String()} {
		res := e.do("GET", "/api/v1/products/"+ref, "", nil)
		if res.Status != 200 {
			t.Fatalf("GET %s = %d: %s", ref, res.Status, res.Raw)
		}
		variants := res.data()["variants"].([]any)
		if len(variants) != 1 || variants[0].(map[string]any)["size"] != "M" {
			t.Errorf("GET %s: public variants = %v, want only the active one", ref, variants)
		}
		if res.data()["status"] != "active" || res.data()["slug"] != "deri-ceket" {
			t.Errorf("data = %v", res.data())
		}
	}
	for _, ref := range []string{draft.Slug, draft.ID.String(), "yok-boyle-urun", uuid.NewString()} {
		res := e.do("GET", "/api/v1/products/"+ref, "", nil)
		if res.Status != 404 || res.message() != "Product not found" {
			t.Errorf("GET %s = %d %q, want 404", ref, res.Status, res.message())
		}
	}
}

func TestPublicCategories(t *testing.T) {
	e := newEnv(t)
	created := e.admin("POST", "/api/v1/admin/categories", map[string]any{"name": "Dış Giyim"})
	if created.Status != 201 {
		t.Fatalf("create = %d: %s", created.Status, created.Raw)
	}
	parent := created.data()["id"].(string)
	e.admin("POST", "/api/v1/admin/categories", map[string]any{"name": "Ceket", "parent_id": parent})

	list := e.do("GET", "/api/v1/categories", "", nil)
	if list.Status != 200 || len(list.Body["data"].([]any)) != 2 {
		t.Fatalf("list = %d %s", list.Status, list.Raw)
	}
	one := e.do("GET", "/api/v1/categories/dis-giyim", "", nil)
	if one.Status != 200 || one.data()["name"] != "Dış Giyim" || one.data()["parent_id"] != nil {
		t.Errorf("by slug = %d %s", one.Status, one.Raw)
	}
	if res := e.do("GET", "/api/v1/categories/yok", "", nil); res.Status != 404 {
		t.Errorf("unknown slug = %d", res.Status)
	}
}

func TestCacheHeaders(t *testing.T) {
	e := newEnv(t)
	p := e.seedProduct("Deri Ceket", domain.StatusActive, active("M", 1000))
	e.admin("POST", "/api/v1/admin/categories", map[string]any{"name": "Giyim"})

	for _, path := range []string{"/api/v1/products", "/api/v1/products/" + p.Slug, "/api/v1/categories", "/api/v1/categories/giyim"} {
		if cc := e.do("GET", path, "", nil).Header.Get("Cache-Control"); cc != "public, max-age=60" {
			t.Errorf("GET %s Cache-Control = %q", path, cc)
		}
	}
	// Admin and error responses must never be cached publicly.
	for _, res := range []result{
		e.admin("GET", "/api/v1/admin/products", nil),
		e.do("GET", "/api/v1/products/yok", "", nil),
	} {
		if cc := res.Header.Get("Cache-Control"); strings.Contains(cc, "public") {
			t.Errorf("status %d response has Cache-Control %q", res.Status, cc)
		}
	}
}

// ---------------------------------------------------------------- authorization

func TestAdminRoutes_Authorization(t *testing.T) {
	e := newEnv(t)
	id, vid := uuid.NewString(), uuid.NewString()
	routes := []struct{ method, path string }{
		{"POST", "/api/v1/admin/categories"},
		{"PATCH", "/api/v1/admin/categories/" + id},
		{"DELETE", "/api/v1/admin/categories/" + id},
		{"GET", "/api/v1/admin/products"},
		{"POST", "/api/v1/admin/products"},
		{"GET", "/api/v1/admin/products/" + id},
		{"PATCH", "/api/v1/admin/products/" + id},
		{"DELETE", "/api/v1/admin/products/" + id},
		{"POST", "/api/v1/admin/products/" + id + "/variants"},
		{"PATCH", "/api/v1/admin/products/" + id + "/variants/" + vid},
	}
	creds := []struct {
		name   string
		bearer string
		header string // raw Authorization header, when set
		want   int    // 0 = must get past authentication/authorization
	}{
		{name: "no header", want: 401},
		{name: "wrong scheme", header: "Basic dXNlcjpwYXNz", want: 401},
		{name: "empty bearer", header: "Bearer ", want: 401},
		{name: "garbage token", bearer: "not.a.jwt", want: 401},
		{name: "wrong signature", bearer: token(t, auth.RoleAdmin, "another-secret-another-secret-123456", time.Hour), want: 401},
		{name: "expired", bearer: token(t, auth.RoleAdmin, testSecret, -time.Minute), want: 401},
		{name: "customer", bearer: token(t, auth.RoleCustomer, testSecret, time.Hour), want: 403},
		{name: "admin", bearer: token(t, auth.RoleAdmin, testSecret, time.Hour), want: 0},
	}
	for _, r := range routes {
		for _, c := range creds {
			req := httptest.NewRequest(r.method, r.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			switch {
			case c.header != "":
				req.Header.Set("Authorization", c.header)
			case c.bearer != "":
				req.Header.Set("Authorization", "Bearer "+c.bearer)
			}
			got := e.send(req).Status
			if c.want != 0 && got != c.want {
				t.Errorf("%s %s [%s] = %d, want %d", r.method, r.path, c.name, got, c.want)
			}
			if c.want == 0 && (got == 401 || got == 403) {
				t.Errorf("%s %s [%s] = %d, admin must pass authorization", r.method, r.path, c.name, got)
			}
		}
	}
}

func TestAdminReads_AreNotPubliclyAccessible(t *testing.T) {
	e := newEnv(t)
	draft := e.seedProduct("Taslak", domain.StatusDraft, active("M", 100))
	if res := e.do("GET", "/api/v1/admin/products/"+draft.ID.String(), "", nil); res.Status != 401 {
		t.Errorf("anonymous admin read = %d", res.Status)
	}
	res := e.admin("GET", "/api/v1/admin/products/"+draft.ID.String(), nil)
	if res.Status != 200 || res.data()["status"] != "draft" {
		t.Errorf("admin read = %d %s", res.Status, res.Raw)
	}
}

func TestAdminList_StatusFilter(t *testing.T) {
	e := newEnv(t)
	e.seedProduct("Canli", domain.StatusActive, active("M", 100))
	e.seedProduct("Taslak", domain.StatusDraft, active("M", 100))

	all := e.admin("GET", "/api/v1/admin/products", nil)
	if all.data()["total"] != float64(2) || e.store.lastFilter.Status != nil {
		t.Errorf("admin list without status: total=%v status=%v", all.data()["total"], e.store.lastFilter.Status)
	}
	drafts := e.admin("GET", "/api/v1/admin/products?status=draft", nil)
	if drafts.data()["total"] != float64(1) {
		t.Errorf("draft filter total = %v", drafts.data()["total"])
	}
	if res := e.admin("GET", "/api/v1/admin/products?status=bogus", nil); res.Status != 400 {
		t.Errorf("bad status filter = %d, want 400", res.Status)
	}
}

// ---------------------------------------------------------------- validation

func TestCreateProduct_Validation(t *testing.T) {
	cat := uuid.NewString()
	manyImages := make([]map[string]any, 11)
	for i := range manyImages {
		manyImages[i] = map[string]any{"url": "https://img.test/x.jpg"}
	}

	tests := []struct {
		name      string
		body      any
		status    int
		wantField string // key expected in "fields"
	}{
		{"empty object", map[string]any{}, 422, "name"},
		{"name too short", map[string]any{"name": "x", "category_id": cat}, 422, "name"},
		{"missing category", map[string]any{"name": "Valid Name"}, 422, "category_id"},
		{"category not a uuid", map[string]any{"name": "Valid Name", "category_id": "nope"}, 422, "category_id"},
		{"unknown status", map[string]any{"name": "Valid Name", "category_id": cat, "status": "published"}, 422, "status"},
		{"javascript image url", map[string]any{"name": "Valid Name", "category_id": cat, "images": []map[string]any{{"url": "https://ok.test/a.jpg"}, {"url": "javascript:alert(1)"}}}, 422, "images[1].url"},
		{"image url missing", map[string]any{"name": "Valid Name", "category_id": cat, "images": []map[string]any{{"alt": "x"}}}, 422, "images[0].url"},
		{"too many images", map[string]any{"name": "Valid Name", "category_id": cat, "images": manyImages}, 422, "images"},
		{"zero price", map[string]any{"name": "Valid Name", "category_id": cat, "variants": []map[string]any{{"sku": "AB-1", "price_cents": 0}}}, 422, "variants[0].price_cents"},
		{"negative price", map[string]any{"name": "Valid Name", "category_id": cat, "variants": []map[string]any{{"sku": "AB-1", "price_cents": -5}}}, 422, "variants[0].price_cents"},
		{"price as string", `{"name":"Valid Name","category_id":"` + cat + `","variants":[{"sku":"AB-1","price_cents":"10"}]}`, 400, ""},
		{"sku too short", map[string]any{"name": "Valid Name", "category_id": cat, "variants": []map[string]any{{"sku": "A", "price_cents": 10}}}, 422, "variants[0].sku"},
		{"malformed json", `{"name": `, 400, ""},
		{"empty body", "", 400, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			res := e.admin("POST", "/api/v1/admin/products", tt.body)
			if res.Status != tt.status {
				t.Fatalf("status = %d, want %d: %s", res.Status, tt.status, res.Raw)
			}
			if res.Body["success"] != false {
				t.Errorf("success = %v", res.Body["success"])
			}
			if tt.wantField != "" {
				if _, ok := res.fields()[tt.wantField]; !ok {
					t.Errorf("fields = %v, want key %q", res.fields(), tt.wantField)
				}
			}
			if len(e.store.products) != 0 {
				t.Error("invalid request created a product")
			}
		})
	}
}

func TestCreateProduct_RejectsInvalidSKUFromService(t *testing.T) {
	e := newEnv(t)
	res := e.admin("POST", "/api/v1/admin/products", map[string]any{
		"name": "Valid Name", "category_id": uuid.NewString(),
		"variants": []map[string]any{{"sku": "has space!", "price_cents": 100}},
	})
	if res.Status != 422 || !strings.Contains(res.message(), "SKU") {
		t.Errorf("got %d %s", res.Status, res.Raw)
	}
}

func TestInvalidPathIDs(t *testing.T) {
	e := newEnv(t)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/admin/products/not-a-uuid"},
		{"PATCH", "/api/v1/admin/products/123"},
		{"DELETE", "/api/v1/admin/products/%27%3B%20DROP%20TABLE%20products%3B--"},
		{"POST", "/api/v1/admin/products/xyz/variants"},
		{"PATCH", "/api/v1/admin/products/" + uuid.NewString() + "/variants/nope"},
		{"PATCH", "/api/v1/admin/categories/nope"},
		{"DELETE", "/api/v1/admin/categories/nope"},
	} {
		if res := e.admin(tc.method, tc.path, map[string]any{}); res.Status != 400 {
			t.Errorf("%s %s = %d, want 400 (%s)", tc.method, tc.path, res.Status, res.Raw)
		}
	}
}

// app.Test short-circuits the network layer, so the body limit is checked over a real socket.
func TestRequestBodyLimit(t *testing.T) {
	e := newEnv(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = e.app.Listener(ln) }()
	t.Cleanup(func() { _ = e.app.Shutdown() })

	// A server that rejects an oversized body may close the connection while the client is still
	// writing it; the client then sees a write error instead of the 413. Both mean "rejected".
	post := func(body string) (status int, err error) {
		req, _ := http.NewRequest("POST", "http://"+ln.Addr().String()+"/api/v1/admin/products", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken(t))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0, err
		}
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode, nil
	}
	if got, err := post(`{"name":"` + strings.Repeat("a", 300*1024) + `"}`); err == nil && got != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body status = %d, want 413 (or a closed connection)", got)
	}
	if got, err := post(`{}`); err != nil || got != http.StatusUnprocessableEntity {
		t.Errorf("small body status = %d, err = %v, want 422", got, err)
	}
}

// ---------------------------------------------------------------- admin happy paths

func TestCreateProduct_Success(t *testing.T) {
	e := newEnv(t)
	cat := e.admin("POST", "/api/v1/admin/categories", map[string]any{"name": "Giyim"}).data()["id"].(string)

	res := e.admin("POST", "/api/v1/admin/products", map[string]any{
		"name": "  Çizgili Gömlek ", "brand": "Rovena", "category_id": cat,
		"images":   []map[string]any{{"url": "https://img.test/a.jpg", "alt": "ön"}},
		"variants": []map[string]any{{"sku": "gml-1", "size": "M", "color": "Mavi", "price_cents": 89900}},
	})
	if res.Status != 201 {
		t.Fatalf("status = %d: %s", res.Status, res.Raw)
	}
	d := res.data()
	if d["name"] != "Çizgili Gömlek" || d["slug"] != "cizgili-gomlek" || d["status"] != "draft" || d["category_id"] != cat {
		t.Errorf("data = %v", d)
	}
	v := d["variants"].([]any)[0].(map[string]any)
	if v["sku"] != "GML-1" || v["active"] != true || v["currency"] != "TRY" || v["price_cents"] != float64(89900) {
		t.Errorf("variant = %v", v)
	}

	// A draft stays invisible to the storefront until published.
	slug := d["slug"].(string)
	if res := e.do("GET", "/api/v1/products/"+slug, "", nil); res.Status != 404 {
		t.Errorf("draft visible publicly: %d", res.Status)
	}
	pub := e.admin("PATCH", "/api/v1/admin/products/"+d["id"].(string), map[string]any{"status": "active"})
	if pub.Status != 200 || pub.data()["status"] != "active" {
		t.Fatalf("publish = %d %s", pub.Status, pub.Raw)
	}
	if res := e.do("GET", "/api/v1/products/"+slug, "", nil); res.Status != 200 {
		t.Errorf("published product not visible: %d", res.Status)
	}
}

func TestUpdateProduct_PartialAndImages(t *testing.T) {
	e := newEnv(t)
	p := e.seedProduct("Deri Ceket", domain.StatusActive, active("M", 1000))
	path := "/api/v1/admin/products/" + p.ID.String()

	res := e.admin("PATCH", path, map[string]any{"name": "Deri Ceket V2"})
	if res.Status != 200 || res.data()["name"] != "Deri Ceket V2" || res.data()["brand"] != "Rovena" {
		t.Fatalf("rename = %d %s", res.Status, res.Raw)
	}
	if res.data()["slug"] != "deri-ceket" {
		t.Errorf("slug changed on rename: %v", res.data()["slug"])
	}
	if len(res.data()["images"].([]any)) != 1 {
		t.Error("omitting images must keep existing ones")
	}

	cleared := e.admin("PATCH", path, map[string]any{"images": []any{}})
	if cleared.Status != 200 || len(cleared.data()["images"].([]any)) != 0 {
		t.Errorf("images: [] must clear, got %s", cleared.Raw)
	}
	if res := e.admin("PATCH", path, map[string]any{"name": "x"}); res.Status != 422 {
		t.Errorf("short name = %d", res.Status)
	}
	if res := e.admin("PATCH", "/api/v1/admin/products/"+uuid.NewString(), map[string]any{"name": "Valid Name"}); res.Status != 404 {
		t.Errorf("unknown product = %d", res.Status)
	}
}

func TestPublishRules(t *testing.T) {
	e := newEnv(t)
	empty := e.seedProduct("Bos Urun", domain.StatusDraft)
	res := e.admin("PATCH", "/api/v1/admin/products/"+empty.ID.String(), map[string]any{"status": "active"})
	if res.Status != 409 || !strings.Contains(res.message(), "active variant") {
		t.Errorf("publish without variants = %d %s", res.Status, res.Raw)
	}
	res = e.admin("POST", "/api/v1/admin/products", map[string]any{
		"name": "Direkt Aktif", "category_id": uuid.NewString(), "status": "active",
	})
	if res.Status != 409 {
		t.Errorf("create active without variants = %d %s", res.Status, res.Raw)
	}
}

func TestArchiveProduct(t *testing.T) {
	e := newEnv(t)
	p := e.seedProduct("Deri Ceket", domain.StatusActive, active("M", 1000))

	if res := e.admin("DELETE", "/api/v1/admin/products/"+p.ID.String(), nil); res.Status != 204 || len(res.Raw) != 0 {
		t.Fatalf("archive = %d %q", res.Status, res.Raw)
	}
	if res := e.do("GET", "/api/v1/products/"+p.Slug, "", nil); res.Status != 404 {
		t.Errorf("archived product still public: %d", res.Status)
	}
	if got := e.admin("GET", "/api/v1/admin/products/"+p.ID.String(), nil); got.data()["status"] != "archived" {
		t.Errorf("admin view = %s", got.Raw)
	}
	if res := e.admin("DELETE", "/api/v1/admin/products/"+uuid.NewString(), nil); res.Status != 404 {
		t.Errorf("archive unknown = %d", res.Status)
	}
}

func TestVariantEndpoints(t *testing.T) {
	e := newEnv(t)
	p := e.seedProduct("Deri Ceket", domain.StatusActive, active("M", 1000), active("L", 2000))
	base := "/api/v1/admin/products/" + p.ID.String() + "/variants"

	created := e.admin("POST", base, map[string]any{"sku": "jkt-xl", "size": "XL", "price_cents": 3000})
	if created.Status != 201 || created.data()["sku"] != "JKT-XL" {
		t.Fatalf("add = %d %s", created.Status, created.Raw)
	}
	vid := created.data()["id"].(string)

	upd := e.admin("PATCH", base+"/"+vid, map[string]any{"price_cents": 3500, "active": false})
	if upd.Status != 200 || upd.data()["price_cents"] != float64(3500) || upd.data()["active"] != false || upd.data()["sku"] != "JKT-XL" {
		t.Errorf("update = %d %s", upd.Status, upd.Raw)
	}
	if res := e.admin("PATCH", base+"/"+vid, map[string]any{"price_cents": 0}); res.Status != 422 {
		t.Errorf("zero price update = %d", res.Status)
	}
	if res := e.admin("PATCH", base+"/"+uuid.NewString(), map[string]any{"active": true}); res.Status != 404 {
		t.Errorf("unknown variant = %d", res.Status)
	}
	// A variant addressed through another product's URL must not be found.
	other := e.seedProduct("Baska Urun", domain.StatusActive, active("M", 1))
	if res := e.admin("PATCH", "/api/v1/admin/products/"+other.ID.String()+"/variants/"+vid, map[string]any{"active": true}); res.Status != 404 {
		t.Errorf("cross-product variant update = %d, want 404", res.Status)
	}
	if res := e.admin("POST", "/api/v1/admin/products/"+uuid.NewString()+"/variants", map[string]any{"sku": "ZZ-1", "price_cents": 1}); res.Status != 404 {
		t.Errorf("variant on unknown product = %d", res.Status)
	}
}

// ---------------------------------------------------------------- error mapping

func TestDomainErrorsMapToHTTP(t *testing.T) {
	cat := uuid.NewString()
	create := map[string]any{"name": "Valid Name", "category_id": cat}

	tests := []struct {
		name    string
		prepare func(e *env)
		call    func(e *env) result
		status  int
		message string
	}{
		{"duplicate sku", func(e *env) { e.store.createProductErr = domain.ErrSKUTaken },
			func(e *env) result { return e.admin("POST", "/api/v1/admin/products", create) }, 409, "SKU is already in use"},
		{"duplicate slug", func(e *env) { e.store.createProductErr = domain.ErrSlugTaken },
			func(e *env) result { return e.admin("POST", "/api/v1/admin/products", create) }, 409, "Slug is already in use"},
		{"duplicate size and color", func(e *env) { e.store.addVariantErr = domain.ErrVariantExists },
			func(e *env) result {
				p := e.seedProduct("P", domain.StatusDraft)
				return e.admin("POST", "/api/v1/admin/products/"+p.ID.String()+"/variants", map[string]any{"sku": "AB-1", "price_cents": 1})
			}, 409, "A variant with this size and color already exists"},
		{"unknown category", func(e *env) { e.store.createProductErr = domain.ErrCategoryNotFound },
			func(e *env) result { return e.admin("POST", "/api/v1/admin/products", create) }, 404, "Category not found"},
		{"category in use", func(e *env) { e.store.deleteCategoryErr = domain.ErrCategoryInUse },
			func(e *env) result { return e.admin("DELETE", "/api/v1/admin/categories/"+uuid.NewString(), nil) }, 409, "Category still has products or subcategories"},
		{"unknown parent category", func(e *env) {},
			func(e *env) result {
				return e.admin("POST", "/api/v1/admin/categories", map[string]any{"name": "Alt", "parent_id": uuid.NewString()})
			}, 404, "Category not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			tt.prepare(e)
			res := tt.call(e)
			if res.Status != tt.status || res.message() != tt.message {
				t.Errorf("got %d %q, want %d %q", res.Status, res.message(), tt.status, tt.message)
			}
		})
	}
}

func TestUnexpectedErrorsDoNotLeakDetails(t *testing.T) {
	e := newEnv(t)
	e.store.createProductErr = errors.New(`pq: connection to 10.0.0.5 refused (password=hunter2)`)
	res := e.admin("POST", "/api/v1/admin/products", map[string]any{"name": "Valid Name", "category_id": uuid.NewString()})
	if res.Status != 500 {
		t.Fatalf("status = %d", res.Status)
	}
	if res.message() != "Internal server error" || strings.Contains(string(res.Raw), "hunter2") || strings.Contains(string(res.Raw), "10.0.0.5") {
		t.Errorf("body leaks internals: %s", res.Raw)
	}
}

func TestCategoryCRUD(t *testing.T) {
	e := newEnv(t)
	c := e.admin("POST", "/api/v1/admin/categories", map[string]any{"name": "Aksesuar", "description": " Takı ve çanta "})
	if c.Status != 201 || c.data()["slug"] != "aksesuar" || c.data()["description"] != "Takı ve çanta" {
		t.Fatalf("create = %d %s", c.Status, c.Raw)
	}
	id := c.data()["id"].(string)

	if res := e.admin("POST", "/api/v1/admin/categories", map[string]any{"name": "a"}); res.Status != 422 {
		t.Errorf("short name = %d", res.Status)
	}
	if res := e.admin("POST", "/api/v1/admin/categories", map[string]any{"name": "Geçerli", "parent_id": "nope"}); res.Status != 422 {
		t.Errorf("bad parent id = %d", res.Status)
	}
	dup := e.admin("POST", "/api/v1/admin/categories", map[string]any{"name": "Aksesuar"})
	if dup.Status != 201 || dup.data()["slug"] != "aksesuar-2" {
		t.Errorf("duplicate name must get a unique slug, got %s", dup.Raw)
	}

	upd := e.admin("PATCH", "/api/v1/admin/categories/"+id, map[string]any{"name": "Aksesuarlar"})
	if upd.Status != 200 || upd.data()["name"] != "Aksesuarlar" || upd.data()["slug"] != "aksesuar" {
		t.Errorf("rename = %s (slug must stay stable)", upd.Raw)
	}
	if res := e.admin("DELETE", "/api/v1/admin/categories/"+id, nil); res.Status != 204 {
		t.Errorf("delete = %d", res.Status)
	}
	if res := e.admin("DELETE", "/api/v1/admin/categories/"+id, nil); res.Status != 404 {
		t.Errorf("second delete = %d, want 404", res.Status)
	}
}

// ---------------------------------------------------------------- cross-cutting

func TestHealthEndpoints(t *testing.T) {
	e := newEnv(t)
	if res := e.do("GET", "/healthz", "", nil); res.Status != 200 || res.Body["status"] != "ok" {
		t.Errorf("healthz = %d %s", res.Status, res.Raw)
	}
	if res := e.do("GET", "/readyz", "", nil); res.Status != 200 || res.Body["ready"] != true {
		t.Errorf("readyz = %d %s", res.Status, res.Raw)
	}

	e.readyErr = errors.New("dial tcp 10.0.0.5:5432: connection refused")
	res := e.do("GET", "/readyz", "", nil)
	if res.Status != 503 || res.Body["ready"] != false {
		t.Errorf("readyz with failing dependency = %d %s", res.Status, res.Raw)
	}
	if strings.Contains(string(res.Raw), "10.0.0.5") {
		t.Errorf("readyz leaks dependency error: %s", res.Raw)
	}
	if res := e.do("GET", "/healthz", "", nil); res.Status != 200 {
		t.Errorf("liveness must not depend on dependencies, got %d", res.Status)
	}
}

func TestRequestIDAndSecurityHeaders(t *testing.T) {
	e := newEnv(t)
	res := e.do("GET", "/api/v1/products", "", nil)
	if res.Header.Get("X-Request-ID") == "" {
		t.Error("response has no X-Request-ID")
	}
	if res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", res.Header.Get("X-Content-Type-Options"))
	}

	req := httptest.NewRequest("GET", "/api/v1/products", nil)
	req.Header.Set("X-Request-ID", "trace-abc-123")
	if got := e.send(req).Header.Get("X-Request-ID"); got != "trace-abc-123" {
		t.Errorf("incoming request id not propagated, got %q", got)
	}
	// Error responses carry it too, so a failing call can be traced in the logs.
	if got := e.do("GET", "/api/v1/products/nope", "", nil).Header.Get("X-Request-ID"); got == "" {
		t.Error("404 response has no X-Request-ID")
	}
}

func TestCORS(t *testing.T) {
	e := newEnv(t)
	preflight := func(origin string) result {
		req := httptest.NewRequest("OPTIONS", "/api/v1/admin/products", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
		return e.send(req)
	}
	ok := preflight("https://shop.example")
	if ok.Status != 204 || ok.Header.Get("Access-Control-Allow-Origin") != "https://shop.example" {
		t.Errorf("allowed origin: %d, ACAO=%q", ok.Status, ok.Header.Get("Access-Control-Allow-Origin"))
	}
	evil := preflight("https://evil.example")
	if got := evil.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin got ACAO=%q", got)
	}
}

func TestUnknownRoutesAndMethods(t *testing.T) {
	e := newEnv(t)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/api/v1/nothing", 404},
		{"GET", "/api/v2/products", 404},
		{"DELETE", "/api/v1/products", 405},
		{"POST", "/api/v1/products", 405},
		{"PUT", "/api/v1/admin/products/" + uuid.NewString(), 405},
	} {
		res := e.do(tc.method, tc.path, adminToken(t), nil)
		if res.Status != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, res.Status, tc.want)
		}
		if res.Body["success"] != false {
			t.Errorf("%s %s: error body is not the standard envelope: %s", tc.method, tc.path, res.Raw)
		}
	}
}
