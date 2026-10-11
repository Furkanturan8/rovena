package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/inventory/internal/application"
	"github.com/Furkanturan8/rovena/services/inventory/internal/domain"
	transport "github.com/Furkanturan8/rovena/services/inventory/internal/transport/http"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	testSecret      = "test-secret-test-secret-test-secret-1234"
	testInternalKey = "internal-key-internal-key-internal-key"
)

type env struct {
	t     *testing.T
	app   *fiber.App
	store *store

	readyErr error
}

func newEnv(t *testing.T) *env {
	return newEnvWithTTL(t, 15*time.Minute)
}

func newEnvWithTTL(t *testing.T, ttl time.Duration) *env {
	t.Helper()
	e := &env{t: t, store: newStore()}
	e.app = transport.NewApp(transport.Options{
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		AllowedOrigins: "https://shop.example",
		TrustedProxies: []string{"127.0.0.1"},
		Verifier:       auth.NewVerifier([]byte(testSecret)),
		InternalAPIKey: testInternalKey,
		Readiness: map[string]httpx.ReadinessCheck{
			"postgres": func(context.Context) error { return e.readyErr },
		},
	}, transport.Services{
		Stock:        application.NewStockService(stockRepo{e.store}),
		Reservations: application.NewReservationService(reservationRepo{e.store}, ttl),
	})
	return e
}

func token(t *testing.T, role, secret string, expiresIn time.Duration) string {
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

type request struct {
	method      string
	path        string
	body        any
	bearer      string
	internalKey string
}

type result struct {
	status int
	body   map[string]any
}

func (e *env) do(r request) result {
	e.t.Helper()
	var body io.Reader
	if r.body != nil {
		raw, err := json.Marshal(r.body)
		if err != nil {
			e.t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(r.method, r.path, body)
	if r.body != nil {
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	}
	if r.bearer != "" {
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+r.bearer)
	}
	if r.internalKey != "" {
		req.Header.Set(transport.InternalKeyHeader, r.internalKey)
	}

	res, err := e.app.Test(req, 5000)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()

	out := result{status: res.StatusCode}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

// admin sends an authenticated admin request.
func (e *env) admin(method, path string, body any) result {
	e.t.Helper()
	return e.do(request{method: method, path: path, body: body, bearer: token(e.t, auth.RoleAdmin, testSecret, time.Hour)})
}

// internal sends a request with the correct service-to-service key.
func (e *env) internal(method, path string, body any) result {
	e.t.Helper()
	return e.do(request{method: method, path: path, body: body, internalKey: testInternalKey})
}

func (r result) data() map[string]any {
	data, _ := r.body["data"].(map[string]any)
	return data
}

func (r result) items() []any {
	items, _ := r.data()["items"].([]any)
	return items
}

func (r result) list() []any {
	items, _ := r.body["data"].([]any)
	return items
}

func (r result) fields() map[string]any {
	fields, _ := r.body["fields"].(map[string]any)
	return fields
}

func num(v any) int {
	f, _ := v.(float64)
	return int(f)
}

func (e *env) expect(r result, status int, context string) result {
	e.t.Helper()
	if r.status != status {
		e.t.Fatalf("%s: status = %d, want %d (body %v)", context, r.status, status, r.body)
	}
	return r
}

// ---- health ----

func TestHealthAndReadiness(t *testing.T) {
	e := newEnv(t)
	e.expect(e.do(request{method: http.MethodGet, path: "/healthz"}), http.StatusOK, "healthz")
	e.expect(e.do(request{method: http.MethodGet, path: "/readyz"}), http.StatusOK, "readyz")

	e.readyErr = context.DeadlineExceeded
	e.expect(e.do(request{method: http.MethodGet, path: "/readyz"}), http.StatusServiceUnavailable, "readyz with a broken database")
}

// ---- public availability ----

func TestAvailabilityIsPublic(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(7)

	r := e.expect(e.do(request{method: http.MethodGet, path: "/api/v1/stock?variant_ids=" + variantID.String()}), http.StatusOK, "availability")
	items := r.list()
	if len(items) != 1 {
		t.Fatalf("items = %v, want 1", items)
	}
	entry := items[0].(map[string]any)
	if num(entry["available"]) != 7 || entry["in_stock"] != true {
		t.Fatalf("entry = %v, want available 7 and in_stock true", entry)
	}
	// on_hand and reserved are warehouse details; the storefront does not see them.
	if _, leaked := entry["on_hand"]; leaked {
		t.Fatalf("public response leaks warehouse detail: %v", entry)
	}
}

func TestAvailabilityIsNotCached(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stock?variant_ids="+variantID.String(), nil)
	res, err := e.app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if got := res.Header.Get(fiber.HeaderCacheControl); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestAvailabilityValidation(t *testing.T) {
	e := newEnv(t)
	tests := []struct{ name, query string }{
		{"missing", ""},
		{"empty", "?variant_ids="},
		{"not a uuid", "?variant_ids=not-a-uuid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e.expect(e.do(request{method: http.MethodGet, path: "/api/v1/stock" + tc.query}), http.StatusBadRequest, tc.name)
		})
	}
}

func TestAvailabilityOfUntrackedVariant(t *testing.T) {
	e := newEnv(t)
	r := e.expect(e.do(request{method: http.MethodGet, path: "/api/v1/stock?variant_ids=" + uuid.NewString()}), http.StatusOK, "availability")
	entry := r.list()[0].(map[string]any)
	if entry["in_stock"] != false || num(entry["available"]) != 0 {
		t.Fatalf("entry = %v, want out of stock", entry)
	}
}

// ---- admin authorization ----

func TestAdminEndpointsRequireAnAdminToken(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(3)
	paths := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/stock"},
		{http.MethodPost, "/api/v1/admin/stock"},
		{http.MethodGet, "/api/v1/admin/stock/" + variantID.String()},
		{http.MethodPost, "/api/v1/admin/stock/" + variantID.String() + "/receipts"},
		{http.MethodPost, "/api/v1/admin/stock/" + variantID.String() + "/adjustments"},
		{http.MethodGet, "/api/v1/admin/stock/" + variantID.String() + "/movements"},
	}
	for _, p := range paths {
		t.Run(p.method+" "+p.path, func(t *testing.T) {
			e.expect(e.do(request{method: p.method, path: p.path}), http.StatusUnauthorized, "no token")
			e.expect(e.do(request{
				method: p.method, path: p.path,
				bearer: token(t, auth.RoleCustomer, testSecret, time.Hour),
			}), http.StatusForbidden, "customer token")
			e.expect(e.do(request{
				method: p.method, path: p.path,
				bearer: token(t, auth.RoleAdmin, "a-different-secret-that-is-long-enough", time.Hour),
			}), http.StatusUnauthorized, "token signed with another secret")
			e.expect(e.do(request{
				method: p.method, path: p.path,
				bearer: token(t, auth.RoleAdmin, testSecret, -time.Minute),
			}), http.StatusUnauthorized, "expired token")
		})
	}
}

// ---- admin stock management ----

func TestCreateStock(t *testing.T) {
	e := newEnv(t)
	variantID := uuid.NewString()

	r := e.expect(e.admin(http.MethodPost, "/api/v1/admin/stock", map[string]any{
		"variant_id": variantID, "sku": "tsh-001-m-blk", "on_hand": 12,
	}), http.StatusCreated, "create")
	data := r.data()
	if data["sku"] != "TSH-001-M-BLK" || num(data["available"]) != 12 || num(data["reserved"]) != 0 {
		t.Fatalf("data = %v", data)
	}

	e.expect(e.admin(http.MethodPost, "/api/v1/admin/stock", map[string]any{
		"variant_id": variantID, "sku": "tsh-001-m-red",
	}), http.StatusConflict, "same variant twice")
}

func TestCreateStockValidation(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name string
		body map[string]any
		want int
	}{
		{"missing variant", map[string]any{"sku": "SKU-1"}, http.StatusUnprocessableEntity},
		{"variant not a uuid", map[string]any{"variant_id": "nope", "sku": "SKU-1"}, http.StatusUnprocessableEntity},
		{"missing sku", map[string]any{"variant_id": uuid.NewString()}, http.StatusUnprocessableEntity},
		{"negative on hand", map[string]any{"variant_id": uuid.NewString(), "sku": "SKU-1", "on_hand": -3}, http.StatusUnprocessableEntity},
		{"sku with spaces", map[string]any{"variant_id": uuid.NewString(), "sku": "a b c"}, http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e.expect(e.admin(http.MethodPost, "/api/v1/admin/stock", tc.body), tc.want, tc.name)
		})
	}
}

func TestReceiveAndAdjustStock(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(10)
	base := "/api/v1/admin/stock/" + variantID.String()

	r := e.expect(e.admin(http.MethodPost, base+"/receipts", map[string]any{
		"quantity": 5, "reason": "delivery",
	}), http.StatusOK, "receipt")
	if num(r.data()["on_hand"]) != 15 {
		t.Fatalf("on_hand = %v, want 15", r.data()["on_hand"])
	}

	r = e.expect(e.admin(http.MethodPost, base+"/adjustments", map[string]any{
		"delta": -4, "reason": "damaged in transit",
	}), http.StatusOK, "adjustment")
	if num(r.data()["on_hand"]) != 11 {
		t.Fatalf("on_hand = %v, want 11", r.data()["on_hand"])
	}

	e.expect(e.admin(http.MethodPost, base+"/receipts", map[string]any{"quantity": 0}), http.StatusUnprocessableEntity, "zero receipt")
	e.expect(e.admin(http.MethodPost, base+"/receipts", map[string]any{"quantity": -1}), http.StatusUnprocessableEntity, "negative receipt")
	// Adjustments must carry a reason so the ledger stays explainable.
	e.expect(e.admin(http.MethodPost, base+"/adjustments", map[string]any{"delta": -1}), http.StatusUnprocessableEntity, "adjustment without reason")
	e.expect(e.admin(http.MethodPost, base+"/adjustments", map[string]any{"delta": 0, "reason": "noop"}), http.StatusUnprocessableEntity, "zero adjustment")
	e.expect(e.admin(http.MethodPost, "/api/v1/admin/stock/"+uuid.NewString()+"/receipts", map[string]any{"quantity": 1}), http.StatusNotFound, "unknown variant")
	e.expect(e.admin(http.MethodPost, "/api/v1/admin/stock/not-a-uuid/receipts", map[string]any{"quantity": 1}), http.StatusBadRequest, "malformed variant id")
}

func TestAdjustmentCannotRemoveReservedStock(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(10)
	e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations", map[string]any{
		"order_id": uuid.NewString(),
		"items":    []any{map[string]any{"variant_id": variantID.String(), "quantity": 9}},
	}), http.StatusCreated, "reserve")

	e.expect(e.admin(http.MethodPost, "/api/v1/admin/stock/"+variantID.String()+"/adjustments", map[string]any{
		"delta": -5, "reason": "stock count",
	}), http.StatusConflict, "adjustment below reserved")
}

func TestListStock(t *testing.T) {
	e := newEnv(t)
	e.store.seed(1)
	e.store.seed(40)

	r := e.expect(e.admin(http.MethodGet, "/api/v1/admin/stock?page_size=1", nil), http.StatusOK, "list")
	if num(r.data()["total"]) != 2 || len(r.items()) != 1 {
		t.Fatalf("page = %v, want 1 of 2 items", r.data())
	}

	r = e.expect(e.admin(http.MethodGet, "/api/v1/admin/stock?below=5", nil), http.StatusOK, "low stock")
	if num(r.data()["total"]) != 1 {
		t.Fatalf("low stock total = %v, want 1", r.data()["total"])
	}

	e.expect(e.admin(http.MethodGet, "/api/v1/admin/stock?sort=cheapest", nil), http.StatusBadRequest, "unknown sort")
	e.expect(e.admin(http.MethodGet, "/api/v1/admin/stock?below=0", nil), http.StatusBadRequest, "below zero")
}

func TestMovementLedger(t *testing.T) {
	e := newEnv(t)
	r := e.expect(e.admin(http.MethodPost, "/api/v1/admin/stock", map[string]any{
		"variant_id": uuid.NewString(), "sku": "SKU-LEDGER", "on_hand": 4, "reason": "opening",
	}), http.StatusCreated, "create")
	variantID := r.data()["variant_id"].(string)

	e.expect(e.admin(http.MethodPost, "/api/v1/admin/stock/"+variantID+"/receipts", map[string]any{"quantity": 2}), http.StatusOK, "receipt")

	r = e.expect(e.admin(http.MethodGet, "/api/v1/admin/stock/"+variantID+"/movements", nil), http.StatusOK, "movements")
	if num(r.data()["total"]) != 2 {
		t.Fatalf("ledger total = %v, want 2", r.data()["total"])
	}
	e.expect(e.admin(http.MethodGet, "/api/v1/admin/stock/"+uuid.NewString()+"/movements", nil), http.StatusNotFound, "unknown variant")
}

// ---- internal reservation API ----

func TestReservationEndpointsRequireTheInternalKey(t *testing.T) {
	e := newEnv(t)
	id := uuid.NewString()
	paths := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/internal/reservations"},
		{http.MethodGet, "/api/v1/internal/reservations/" + id},
		{http.MethodGet, "/api/v1/internal/reservations/by-order/" + id},
		{http.MethodPost, "/api/v1/internal/reservations/" + id + "/commit"},
		{http.MethodPost, "/api/v1/internal/reservations/" + id + "/release"},
	}
	for _, p := range paths {
		t.Run(p.method+" "+p.path, func(t *testing.T) {
			e.expect(e.do(request{method: p.method, path: p.path}), http.StatusUnauthorized, "no key")
			e.expect(e.do(request{method: p.method, path: p.path, internalKey: "wrong-key"}), http.StatusUnauthorized, "wrong key")
			// An admin JWT is not a substitute: these endpoints are not user-facing.
			e.expect(e.do(request{
				method: p.method, path: p.path,
				bearer: token(t, auth.RoleAdmin, testSecret, time.Hour),
			}), http.StatusUnauthorized, "admin token instead of the key")
		})
	}
}

func TestReserveCommitFlow(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(10)
	orderID := uuid.NewString()

	r := e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations", map[string]any{
		"order_id": orderID,
		"items":    []any{map[string]any{"variant_id": variantID.String(), "quantity": 3}},
	}), http.StatusCreated, "reserve")
	reservationID := r.data()["id"].(string)
	if r.data()["status"] != string(domain.ReservationPending) {
		t.Fatalf("status = %v, want pending", r.data()["status"])
	}

	// Reserved units disappear from the public availability right away.
	avail := e.expect(e.do(request{method: http.MethodGet, path: "/api/v1/stock?variant_ids=" + variantID.String()}), http.StatusOK, "availability")
	if num(avail.list()[0].(map[string]any)["available"]) != 7 {
		t.Fatalf("available = %v, want 7", avail.list()[0])
	}

	// A retried request returns 200 with the same reservation instead of holding twice.
	again := e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations", map[string]any{
		"order_id": orderID,
		"items":    []any{map[string]any{"variant_id": variantID.String(), "quantity": 3}},
	}), http.StatusOK, "replayed reserve")
	if again.data()["id"] != reservationID {
		t.Fatalf("replay returned a different reservation: %v", again.data())
	}

	byOrder := e.expect(e.internal(http.MethodGet, "/api/v1/internal/reservations/by-order/"+orderID, nil), http.StatusOK, "by order")
	if byOrder.data()["id"] != reservationID {
		t.Fatalf("by-order = %v", byOrder.data())
	}

	r = e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations/"+reservationID+"/commit", nil), http.StatusOK, "commit")
	if r.data()["status"] != string(domain.ReservationCommitted) {
		t.Fatalf("status = %v, want committed", r.data()["status"])
	}
	e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations/"+reservationID+"/commit", nil), http.StatusOK, "repeated commit")

	stock := e.expect(e.admin(http.MethodGet, "/api/v1/admin/stock/"+variantID.String(), nil), http.StatusOK, "stock")
	if num(stock.data()["on_hand"]) != 7 || num(stock.data()["reserved"]) != 0 {
		t.Fatalf("stock = %v, want on_hand 7 / reserved 0", stock.data())
	}

	e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations/"+reservationID+"/release", nil), http.StatusConflict, "release after commit")
}

func TestReleaseFlow(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(5)

	r := e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations", map[string]any{
		"order_id": uuid.NewString(),
		"items":    []any{map[string]any{"variant_id": variantID.String(), "quantity": 5}},
	}), http.StatusCreated, "reserve")
	reservationID := r.data()["id"].(string)

	r = e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations/"+reservationID+"/release", nil), http.StatusOK, "release")
	if r.data()["status"] != string(domain.ReservationReleased) {
		t.Fatalf("status = %v, want released", r.data()["status"])
	}
	e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations/"+reservationID+"/release", nil), http.StatusOK, "repeated release")
	e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations/"+reservationID+"/commit", nil), http.StatusConflict, "commit after release")

	stock := e.expect(e.admin(http.MethodGet, "/api/v1/admin/stock/"+variantID.String(), nil), http.StatusOK, "stock")
	if num(stock.data()["reserved"]) != 0 {
		t.Fatalf("reserved = %v, want 0", stock.data()["reserved"])
	}
}

func TestReserveReportsWhatIsMissing(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(2)

	r := e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations", map[string]any{
		"order_id": uuid.NewString(),
		"items":    []any{map[string]any{"variant_id": variantID.String(), "quantity": 5}},
	}), http.StatusConflict, "insufficient stock")
	fields := r.fields()
	if fields["variant_id"] != variantID.String() || fields["requested"] != "5" || fields["available"] != "2" {
		t.Fatalf("fields = %v, want the failing variant with requested/available", fields)
	}

	r = e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations", map[string]any{
		"order_id": uuid.NewString(),
		"items":    []any{map[string]any{"variant_id": uuid.NewString(), "quantity": 1}},
	}), http.StatusNotFound, "untracked variant")
	if r.fields()["variant_id"] == nil {
		t.Fatalf("fields = %v, want the unknown variant", r.fields())
	}
}

func TestReserveValidation(t *testing.T) {
	e := newEnv(t)
	variantID := e.store.seed(5).String()
	item := map[string]any{"variant_id": variantID, "quantity": 1}

	tests := []struct {
		name string
		body map[string]any
		want int
	}{
		{"missing order", map[string]any{"items": []any{item}}, http.StatusUnprocessableEntity},
		{"order not a uuid", map[string]any{"order_id": "nope", "items": []any{item}}, http.StatusUnprocessableEntity},
		{"no items", map[string]any{"order_id": uuid.NewString(), "items": []any{}}, http.StatusUnprocessableEntity},
		{"items missing", map[string]any{"order_id": uuid.NewString()}, http.StatusUnprocessableEntity},
		{"zero quantity", map[string]any{"order_id": uuid.NewString(), "items": []any{
			map[string]any{"variant_id": variantID, "quantity": 0},
		}}, http.StatusUnprocessableEntity},
		{"duplicate variant", map[string]any{"order_id": uuid.NewString(), "items": []any{item, item}}, http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations", tc.body), tc.want, tc.name)
		})
	}

	e.expect(e.do(request{
		method: http.MethodPost, path: "/api/v1/internal/reservations",
		internalKey: testInternalKey, body: "not-an-object",
	}), http.StatusBadRequest, "malformed body")
}

func TestUnknownReservation(t *testing.T) {
	e := newEnv(t)
	id := uuid.NewString()
	e.expect(e.internal(http.MethodGet, "/api/v1/internal/reservations/"+id, nil), http.StatusNotFound, "get")
	e.expect(e.internal(http.MethodGet, "/api/v1/internal/reservations/by-order/"+id, nil), http.StatusNotFound, "by order")
	e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations/"+id+"/commit", nil), http.StatusNotFound, "commit")
	e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations/"+id+"/release", nil), http.StatusNotFound, "release")
	e.expect(e.internal(http.MethodGet, "/api/v1/internal/reservations/not-a-uuid", nil), http.StatusBadRequest, "malformed id")
}

func TestExpiredReservationCannotBeCommittedAfterSweep(t *testing.T) {
	// The hold is already expired when it is created, as if the customer never came back.
	e := newEnvWithTTL(t, -time.Minute)
	variantID := e.store.seed(3)

	r := e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations", map[string]any{
		"order_id": uuid.NewString(),
		"items":    []any{map[string]any{"variant_id": variantID.String(), "quantity": 3}},
	}), http.StatusCreated, "reserve")
	reservationID := r.data()["id"].(string)

	expirer := application.NewExpirer(reservationRepo{e.store}, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Minute)
	if released, err := expirer.RunOnce(context.Background()); err != nil || released != 1 {
		t.Fatalf("sweep = %d, %v; want 1, nil", released, err)
	}

	r = e.expect(e.internal(http.MethodGet, "/api/v1/internal/reservations/"+reservationID, nil), http.StatusOK, "get")
	if r.data()["status"] != string(domain.ReservationExpired) {
		t.Fatalf("status = %v, want expired", r.data()["status"])
	}
	e.expect(e.internal(http.MethodPost, "/api/v1/internal/reservations/"+reservationID+"/commit", nil), http.StatusConflict, "commit after expiry")

	avail := e.expect(e.do(request{method: http.MethodGet, path: "/api/v1/stock?variant_ids=" + variantID.String()}), http.StatusOK, "availability")
	if num(avail.list()[0].(map[string]any)["available"]) != 3 {
		t.Fatalf("expired hold was not given back: %v", avail.list()[0])
	}
}
