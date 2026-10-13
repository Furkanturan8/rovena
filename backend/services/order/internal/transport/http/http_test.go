package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/order/internal/application"
	"github.com/Furkanturan8/rovena/services/order/internal/domain"
	"github.com/Furkanturan8/rovena/services/order/internal/repository/memory"
	transport "github.com/Furkanturan8/rovena/services/order/internal/transport/http"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/events"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testSecret = "test-secret-test-secret-test-secret-1234"

// ---- fakes for the two services Order calls ----

type catalogStub struct {
	mu       sync.Mutex
	variants map[uuid.UUID]application.VariantInfo
	err      error
}

func (c *catalogStub) Lookup(_ context.Context, ids []uuid.UUID) ([]application.VariantInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	var out []application.VariantInfo
	for _, id := range ids {
		if v, ok := c.variants[id]; ok {
			out = append(out, v)
		}
	}
	return out, nil
}

type inventoryStub struct {
	mu       sync.Mutex
	byOrder  map[uuid.UUID]uuid.UUID
	shortage *application.StockShortage
	err      error
}

func (i *inventoryStub) Reserve(_ context.Context, orderID uuid.UUID, _ []application.ReservationLine) (*application.Reservation, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.err != nil {
		return nil, i.err
	}
	if i.shortage != nil {
		s := *i.shortage
		return nil, &s
	}
	id, ok := i.byOrder[orderID]
	if !ok {
		id = uuid.New()
		i.byOrder[orderID] = id
	}
	return &application.Reservation{ID: id}, nil
}
func (i *inventoryStub) Commit(context.Context, uuid.UUID) error  { return nil }
func (i *inventoryStub) Release(context.Context, uuid.UUID) error { return nil }

// ---- environment ----

type env struct {
	t         *testing.T
	app       *fiber.App
	repo      *memory.Repo
	catalog   *catalogStub
	inventory *inventoryStub
	svc       *application.OrderService
	readyErr  error
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		t: t, repo: memory.New(time.Now),
		catalog:   &catalogStub{variants: map[uuid.UUID]application.VariantInfo{}},
		inventory: &inventoryStub{byOrder: map[uuid.UUID]uuid.UUID{}},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	e.svc = application.NewOrderService(e.repo, e.catalog, e.inventory, log, application.Config{})
	e.app = transport.NewApp(transport.Options{
		Logger:         log,
		AllowedOrigins: "https://shop.example",
		TrustedProxies: []string{"127.0.0.1"},
		Verifier:       auth.NewVerifier([]byte(testSecret)),
		Readiness: map[string]httpx.ReadinessCheck{
			"postgres": func(context.Context) error { return e.readyErr },
		},
	}, transport.Services{
		Carts:   application.NewCartService(e.repo, e.catalog),
		Orders:  e.svc,
		Queries: application.NewQueryService(e.repo),
	})
	return e
}

func (e *env) variant(name string, price int64) uuid.UUID {
	id := uuid.New()
	e.catalog.variants[id] = application.VariantInfo{
		VariantID: id, ProductID: uuid.New(), ProductName: name, SKU: "SKU-" + id.String()[:6],
		Size: "M", Color: "Blue", PriceCents: price, Currency: "TRY", Sellable: true,
	}
	return id
}

type user struct {
	id    uuid.UUID
	token string
}

func (e *env) user(role string) user {
	e.t.Helper()
	id := uuid.New()
	claims := auth.Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: auth.Issuer, Subject: id.String(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		e.t.Fatal(err)
	}
	return user{id: id, token: s}
}

type result struct {
	Status int
	Raw    []byte
	Body   map[string]any
}

func (r result) data() map[string]any { d, _ := r.Body["data"].(map[string]any); return d }

func (e *env) do(method, path string, u *user, headers map[string]string, body any) result {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if u != nil {
		req.Header.Set("Authorization", "Bearer "+u.token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := e.app.Test(req, -1)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	res := result{Status: resp.StatusCode, Raw: raw}
	_ = json.Unmarshal(raw, &res.Body)
	return res
}

func (e *env) putItem(u user, variant uuid.UUID, qty int) result {
	return e.do("PUT", "/api/v1/cart/items/"+variant.String(), &u, nil, map[string]any{"quantity": qty})
}

func (e *env) checkout(u user, key, method string) result {
	h := map[string]string{}
	if key != "" {
		h["Idempotency-Key"] = key
	}
	return e.do("POST", "/api/v1/checkout", &u, h, map[string]any{"payment_method": method, "shipping_address": map[string]any{
		"full_name": "Jane Doe", "phone": "05320000000", "line1": "Moda Cad. 1", "district": "Kadikoy", "city": "Istanbul",
	}})
}

// ---------------------------------------------------------------- access control

func TestEveryRouteRequiresAToken(t *testing.T) {
	e := newEnv(t)
	id := uuid.NewString()
	for _, r := range [][2]string{
		{"GET", "/api/v1/cart"}, {"DELETE", "/api/v1/cart"},
		{"PUT", "/api/v1/cart/items/" + id}, {"DELETE", "/api/v1/cart/items/" + id},
		{"POST", "/api/v1/checkout"}, {"GET", "/api/v1/orders"}, {"GET", "/api/v1/orders/" + id},
		{"GET", "/api/v1/admin/orders"}, {"GET", "/api/v1/admin/orders/" + id},
	} {
		if res := e.do(r[0], r[1], nil, nil, nil); res.Status != 401 {
			t.Errorf("%s %s without token: %d", r[0], r[1], res.Status)
		}
	}
}

func TestAdminRoutesRejectCustomers(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	for _, path := range []string{"/api/v1/admin/orders", "/api/v1/admin/orders/" + uuid.NewString()} {
		if res := e.do("GET", path, &c, nil, nil); res.Status != 403 {
			t.Errorf("GET %s as customer: %d", path, res.Status)
		}
	}
}

func TestHealthAndReadiness(t *testing.T) {
	e := newEnv(t)
	if res := e.do("GET", "/healthz", nil, nil, nil); res.Status != 200 {
		t.Fatalf("healthz = %d", res.Status)
	}
	e.readyErr = io.ErrUnexpectedEOF
	if res := e.do("GET", "/readyz", nil, nil, nil); res.Status != 503 {
		t.Fatalf("readyz with a broken database = %d", res.Status)
	}
}

// ---------------------------------------------------------------- cart

func TestCart_EmptyCartIsAnEmptyArray(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	res := e.do("GET", "/api/v1/cart", &c, nil, nil)
	if res.Status != 200 || !strings.Contains(string(res.Raw), `"items":[]`) {
		t.Fatalf("%d %s", res.Status, res.Raw)
	}
}

func TestCart_ClientPriceIsIgnored(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	v := e.variant("Shirt", 1500)

	res := e.do("PUT", "/api/v1/cart/items/"+v.String(), &c, nil,
		map[string]any{"quantity": 2, "price_cents": 1, "unit_price_cents": 1, "total_cents": 1})
	if res.Status != 200 {
		t.Fatalf("%d %s", res.Status, res.Raw)
	}
	if res.data()["total_cents"].(float64) != 3000 {
		t.Fatalf("total = %v; the price must come from Catalog", res.data()["total_cents"])
	}
}

func TestCart_Validation(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	v := e.variant("Shirt", 100)
	for name, tc := range map[string]struct {
		path string
		body any
		want int
	}{
		"zero quantity":  {"/api/v1/cart/items/" + v.String(), map[string]any{"quantity": 0}, 422},
		"huge quantity":  {"/api/v1/cart/items/" + v.String(), map[string]any{"quantity": 100}, 422},
		"no quantity":    {"/api/v1/cart/items/" + v.String(), map[string]any{}, 422},
		"bad json":       {"/api/v1/cart/items/" + v.String(), "{nope", 400},
		"bad variant id": {"/api/v1/cart/items/not-a-uuid", map[string]any{"quantity": 1}, 400},
	} {
		if res := e.do("PUT", tc.path, &c, nil, tc.body); res.Status != tc.want {
			t.Errorf("%s: %d %s, want %d", name, res.Status, res.Raw, tc.want)
		}
	}
}

func TestCart_UnsellableVariantIsNamed(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	ghost := uuid.New()
	res := e.putItem(c, ghost, 1)
	fields, _ := res.Body["fields"].(map[string]any)
	if res.Status != 409 || fields["variant_id"] != ghost.String() {
		t.Fatalf("%d %s", res.Status, res.Raw)
	}
}

func TestCart_CustomersDoNotSeeEachOthersCarts(t *testing.T) {
	e := newEnv(t)
	a, b := e.user(auth.RoleCustomer), e.user(auth.RoleCustomer)
	e.putItem(a, e.variant("Shirt", 100), 1)
	res := e.do("GET", "/api/v1/cart", &b, nil, nil)
	if items := res.data()["items"].([]any); len(items) != 0 {
		t.Fatalf("customer B sees %d items of customer A", len(items))
	}
}

func TestCart_RemoveAndClear(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	v1, v2 := e.variant("A", 100), e.variant("B", 200)
	e.putItem(c, v1, 1)
	e.putItem(c, v2, 1)

	res := e.do("DELETE", "/api/v1/cart/items/"+v1.String(), &c, nil, nil)
	if res.Status != 200 || len(res.data()["items"].([]any)) != 1 {
		t.Fatalf("%d %s", res.Status, res.Raw)
	}
	if res := e.do("DELETE", "/api/v1/cart", &c, nil, nil); res.Status != 204 {
		t.Fatalf("clear = %d", res.Status)
	}
}

func TestCart_CatalogOutageIs503(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	e.putItem(c, e.variant("A", 1), 1)
	e.catalog.err = domain.ErrDependencyUnavailable
	if res := e.do("GET", "/api/v1/cart", &c, nil, nil); res.Status != 503 {
		t.Fatalf("status = %d, want 503", res.Status)
	}
}

// ---------------------------------------------------------------- checkout

func TestCheckout_RequiresAnIdempotencyKey(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	e.putItem(c, e.variant("Shirt", 100), 1)
	if res := e.checkout(c, "", "tok_visa"); res.Status != 400 {
		t.Fatalf("status = %d", res.Status)
	}
	if res := e.checkout(c, strings.Repeat("k", 65), "tok_visa"); res.Status != 400 {
		t.Fatalf("long key: status = %d", res.Status)
	}
}

func TestCheckout_NeedsAPaymentMethod(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	e.putItem(c, e.variant("Shirt", 100), 1)
	if res := e.checkout(c, "k", ""); res.Status != 422 {
		t.Fatalf("status = %d", res.Status)
	}
}

func TestCheckout_CreatesThenReplays(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	e.putItem(c, e.variant("Shirt", 1500), 2)

	first := e.checkout(c, "key-1", "tok_visa")
	if first.Status != 201 {
		t.Fatalf("first: %d %s", first.Status, first.Raw)
	}
	d := first.data()
	if d["status"] != "awaiting_payment" || d["total_cents"].(float64) != 3000 || d["currency"] != "TRY" {
		t.Fatalf("order = %v", d)
	}

	second := e.checkout(c, "key-1", "tok_visa")
	if second.Status != 200 || second.data()["id"] != d["id"] {
		t.Fatalf("replay: %d %s", second.Status, second.Raw)
	}
	if n := len(e.repo.Orders); n != 1 {
		t.Fatalf("%d orders exist", n)
	}
}

func TestCheckout_KeyReusedWithAnotherMethodIs422(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	e.putItem(c, e.variant("Shirt", 100), 1)
	e.checkout(c, "k", "tok_visa")
	if res := e.checkout(c, "k", "tok_other"); res.Status != 422 {
		t.Fatalf("status = %d", res.Status)
	}
}

func TestCheckout_EmptyCartIs422(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	if res := e.checkout(c, "k", "tok_visa"); res.Status != 422 {
		t.Fatalf("status = %d", res.Status)
	}
}

func TestCheckout_OutOfStockAnswersWithTheCancelledOrder(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	v := e.variant("Shirt", 100)
	e.putItem(c, v, 3)
	e.inventory.shortage = &application.StockShortage{VariantID: v, Requested: 3, Available: 1}

	res := e.checkout(c, "k", "tok_visa")
	d := res.data()
	if res.Status != 201 || d["status"] != "cancelled" || d["failure_code"] != "out_of_stock" {
		t.Fatalf("%d %s", res.Status, res.Raw)
	}
}

func TestCheckout_InventoryOutageStillAnswersWithAPendingOrder(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	e.putItem(c, e.variant("Shirt", 100), 1)
	e.inventory.err = io.ErrUnexpectedEOF

	res := e.checkout(c, "k", "tok_visa")
	if res.Status != 201 || res.data()["status"] != "pending" {
		t.Fatalf("%d %s", res.Status, res.Raw)
	}
}

func TestCheckout_CatalogOutageIs503AndKeepsTheCart(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	e.putItem(c, e.variant("Shirt", 100), 1)
	e.catalog.err = domain.ErrDependencyUnavailable

	if res := e.checkout(c, "k", "tok_visa"); res.Status != 503 {
		t.Fatalf("status = %d", res.Status)
	}
	items, _ := e.repo.GetCart(context.Background(), c.id)
	if len(items) != 1 || len(e.repo.Orders) != 0 {
		t.Fatal("a failed checkout must neither create an order nor empty the cart")
	}
}

func TestCheckout_UnavailableItemIsNamed(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	v := e.variant("Shirt", 100)
	e.putItem(c, v, 1)
	e.catalog.variants[v] = func() application.VariantInfo { x := e.catalog.variants[v]; x.Sellable = false; return x }()

	res := e.checkout(c, "k", "tok_visa")
	fields, _ := res.Body["fields"].(map[string]any)
	if res.Status != 409 || fields["variant_id"] != v.String() {
		t.Fatalf("%d %s", res.Status, res.Raw)
	}
}

// ---------------------------------------------------------------- orders

func TestOrders_CustomerSeesOnlyOwnOrders(t *testing.T) {
	e := newEnv(t)
	a, b := e.user(auth.RoleCustomer), e.user(auth.RoleCustomer)
	e.putItem(a, e.variant("A-item", 100), 1)
	e.putItem(b, e.variant("B-item", 200), 1)
	orderA := e.checkout(a, "ka", "tok_visa").data()["id"].(string)
	e.checkout(b, "kb", "tok_visa")

	res := e.do("GET", "/api/v1/orders", &a, nil, nil)
	items := res.data()["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != orderA || res.data()["total"].(float64) != 1 {
		t.Fatalf("%s", res.Raw)
	}
}

func TestOrders_AnotherCustomersOrderIs404(t *testing.T) {
	e := newEnv(t)
	a, b := e.user(auth.RoleCustomer), e.user(auth.RoleCustomer)
	e.putItem(a, e.variant("Shirt", 100), 1)
	id := e.checkout(a, "k", "tok_visa").data()["id"].(string)

	if res := e.do("GET", "/api/v1/orders/"+id, &b, nil, nil); res.Status != 404 {
		t.Fatalf("status = %d; another customer's order must look like it does not exist", res.Status)
	}
	if res := e.do("GET", "/api/v1/orders/"+id, &a, nil, nil); res.Status != 200 {
		t.Fatalf("own order: %d", res.Status)
	}
	if res := e.do("GET", "/api/v1/orders/"+uuid.NewString(), &a, nil, nil); res.Status != 404 {
		t.Fatalf("unknown order: %d", res.Status)
	}
	if res := e.do("GET", "/api/v1/orders/nope", &a, nil, nil); res.Status != 400 {
		t.Fatalf("bad id: %d", res.Status)
	}
}

func TestOrders_CustomerViewHidesInternals(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	e.putItem(c, e.variant("Shirt", 100), 1)
	created := e.checkout(c, "secret-key-123", "tok_visa")
	id := created.data()["id"].(string)

	detail := e.do("GET", "/api/v1/orders/"+id, &c, nil, nil)
	reservationID := e.inventory.byOrder[uuid.MustParse(id)].String()
	for name, raw := range map[string]string{"checkout": string(created.Raw), "detail": string(detail.Raw)} {
		for _, secret := range []string{"tok_visa", "secret-key-123", reservationID, "reservation", "payment_requests", "customer_id"} {
			if strings.Contains(raw, secret) {
				t.Errorf("%s response leaks %q: %s", name, secret, raw)
			}
		}
	}
	history := detail.data()["history"].([]any)
	if len(history) != 2 {
		t.Fatalf("history = %v", history)
	}
}

func TestAdmin_SeesEveryOrderWithBookkeeping(t *testing.T) {
	e := newEnv(t)
	admin := e.user(auth.RoleAdmin)
	a, b := e.user(auth.RoleCustomer), e.user(auth.RoleCustomer)
	e.putItem(a, e.variant("A-item", 100), 1)
	e.putItem(b, e.variant("B-item", 200), 1)
	idA := e.checkout(a, "ka", "tok_visa").data()["id"].(string)
	e.checkout(b, "kb", "tok_visa")

	res := e.do("GET", "/api/v1/admin/orders", &admin, nil, nil)
	if res.data()["total"].(float64) != 2 {
		t.Fatalf("%s", res.Raw)
	}
	res = e.do("GET", "/api/v1/admin/orders?customer_id="+a.id.String(), &admin, nil, nil)
	if res.data()["total"].(float64) != 1 {
		t.Fatalf("customer filter: %s", res.Raw)
	}
	res = e.do("GET", "/api/v1/admin/orders?status=confirmed", &admin, nil, nil)
	if res.data()["total"].(float64) != 0 {
		t.Fatalf("status filter: %s", res.Raw)
	}

	d := e.do("GET", "/api/v1/admin/orders/"+idA, &admin, nil, nil).data()
	if d["customer_id"] != a.id.String() || d["reservation_state"] != "held" || d["payment_requests"].(float64) != 1 {
		t.Fatalf("detail = %v", d)
	}
	if h := d["history"].([]any); len(h) != 2 || h[1].(map[string]any)["reason"] == "" {
		t.Fatalf("history = %v", h)
	}
}

func TestAdmin_RejectsBadFilters(t *testing.T) {
	e := newEnv(t)
	admin := e.user(auth.RoleAdmin)
	for _, q := range []string{"status=nope", "customer_id=nope"} {
		if res := e.do("GET", "/api/v1/admin/orders?"+q, &admin, nil, nil); res.Status != http.StatusBadRequest {
			t.Errorf("%s: %d", q, res.Status)
		}
	}
}

// ---------------------------------------------------------------- cancelling

func TestCancel_OwnerCancelsAndRepeatIsHarmless(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	e.putItem(c, e.variant("Shirt", 100), 1)
	id := e.checkout(c, "k", "tok_visa").data()["id"].(string)

	for i := 0; i < 2; i++ {
		res := e.do("POST", "/api/v1/orders/"+id+"/cancel", &c, nil, nil)
		d := res.data()
		if res.Status != 200 || d["status"] != "cancelled" || d["failure_code"] != "cancelled_by_customer" {
			t.Fatalf("round %d: %d %s", i, res.Status, res.Raw)
		}
	}
	if _, has := e.do("GET", "/api/v1/orders/"+id, &c, nil, nil).data()["refund"]; has {
		t.Fatal("no refund field when nothing was paid")
	}
}

func TestCancel_AccessRules(t *testing.T) {
	e := newEnv(t)
	owner, other := e.user(auth.RoleCustomer), e.user(auth.RoleCustomer)
	e.putItem(owner, e.variant("Shirt", 100), 1)
	id := e.checkout(owner, "k", "tok_visa").data()["id"].(string)

	if res := e.do("POST", "/api/v1/orders/"+id+"/cancel", nil, nil, nil); res.Status != 401 {
		t.Errorf("no token: %d", res.Status)
	}
	if res := e.do("POST", "/api/v1/orders/"+id+"/cancel", &other, nil, nil); res.Status != 404 {
		t.Errorf("another customer: %d, want 404", res.Status)
	}
	if res := e.do("POST", "/api/v1/orders/nope/cancel", &owner, nil, nil); res.Status != 400 {
		t.Errorf("bad id: %d", res.Status)
	}
	if res := e.do("POST", "/api/v1/orders/"+uuid.NewString()+"/cancel", &owner, nil, nil); res.Status != 404 {
		t.Errorf("unknown order: %d", res.Status)
	}
	if got := e.do("GET", "/api/v1/orders/"+id, &owner, nil, nil).data()["status"]; got != "awaiting_payment" {
		t.Errorf("the refused requests must leave the order alone, status = %v", got)
	}
}

func TestCancel_ConfirmedOrderIs409(t *testing.T) {
	e := newEnv(t)
	c := e.user(auth.RoleCustomer)
	e.putItem(c, e.variant("Shirt", 100), 1)
	created := e.checkout(c, "k", "tok_visa").data()
	oid := uuid.MustParse(created["id"].(string))
	if err := e.svc.OnPaymentSucceeded(context.Background(), events.PaymentSucceededPayload{
		PaymentID: uuid.New(), OrderID: oid, AmountCents: 100, Currency: "TRY", ProviderRef: "r",
	}); err != nil {
		t.Fatal(err)
	}
	res := e.do("POST", "/api/v1/orders/"+oid.String()+"/cancel", &c, nil, nil)
	if res.Status != 409 {
		t.Fatalf("%d %s", res.Status, res.Raw)
	}
}

func TestRefundStateIsShownToTheCustomerAndInDetailToAdmins(t *testing.T) {
	e := newEnv(t)
	c, admin := e.user(auth.RoleCustomer), e.user(auth.RoleAdmin)
	e.putItem(c, e.variant("Shirt", 100), 1)
	oid := uuid.MustParse(e.checkout(c, "k", "tok_visa").data()["id"].(string))
	e.do("POST", "/api/v1/orders/"+oid.String()+"/cancel", &c, nil, nil)
	// the charge went through before the cancellation was noticed
	if err := e.svc.OnPaymentSucceeded(context.Background(), events.PaymentSucceededPayload{
		PaymentID: uuid.New(), OrderID: oid, AmountCents: 100, Currency: "TRY", ProviderRef: "r",
	}); err != nil {
		t.Fatal(err)
	}

	mine := e.do("GET", "/api/v1/orders/"+oid.String(), &c, nil, nil).data()
	if mine["status"] != "cancelled" || mine["refund"] != "pending" {
		t.Fatalf("customer view: %v", mine)
	}
	adm := e.do("GET", "/api/v1/admin/orders/"+oid.String(), &admin, nil, nil).data()
	if adm["refund_state"] != "requested" || adm["refund_requests"].(float64) != 1 {
		t.Fatalf("admin view: %v", adm)
	}

	if err := e.svc.OnPaymentRefunded(context.Background(), events.PaymentRefundedPayload{
		PaymentID: uuid.New(), OrderID: oid, AmountCents: 100, Currency: "TRY", RefundRef: "rf",
	}); err != nil {
		t.Fatal(err)
	}
	if got := e.do("GET", "/api/v1/orders/"+oid.String(), &c, nil, nil).data()["refund"]; got != "refunded" {
		t.Fatalf("refund = %v", got)
	}
}

func TestCheckout_NeedsAShippingAddress(t *testing.T) {
	e := newEnv(t)
	c := e.user("customer")
	h := map[string]string{"Idempotency-Key": "k"}
	res := e.do("POST", "/api/v1/checkout", &c, h, map[string]any{"payment_method": "tok_visa"})
	if res.Status != 422 {
		t.Fatalf("status = %d, want 422: %s", res.Status, res.Body)
	}
}
