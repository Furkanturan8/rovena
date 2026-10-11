package http_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/services/payment/internal/application"
	"github.com/Furkanturan8/rovena/services/payment/internal/domain"
	"github.com/Furkanturan8/rovena/services/payment/internal/repository"
	transport "github.com/Furkanturan8/rovena/services/payment/internal/transport/http"
	"github.com/Furkanturan8/rovena/shared/auth"
	"github.com/Furkanturan8/rovena/shared/httpx"
	"github.com/Furkanturan8/rovena/shared/messaging"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	testSecret      = "test-secret-test-secret-test-secret-1234"
	testInternalKey = "internal-key-internal-key-internal-key"
)

// readRepo is a read-only repository: the HTTP API never writes payments.
type readRepo struct {
	payments []domain.Payment
}

var _ repository.PaymentRepository = (*readRepo)(nil)

func (r *readRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Payment, error) {
	for i := range r.payments {
		if r.payments[i].ID == id {
			p := r.payments[i]
			return &p, nil
		}
	}
	return nil, domain.ErrPaymentNotFound
}

func (r *readRepo) LatestByOrder(_ context.Context, orderID uuid.UUID) (*domain.Payment, error) {
	var latest *domain.Payment
	for i := range r.payments {
		if r.payments[i].OrderID == orderID && (latest == nil || r.payments[i].CreatedAt.After(latest.CreatedAt)) {
			latest = &r.payments[i]
		}
	}
	if latest == nil {
		return nil, domain.ErrPaymentNotFound
	}
	p := *latest
	return &p, nil
}

func (r *readRepo) List(_ context.Context, f domain.Filter) ([]domain.Payment, int, error) {
	var all []domain.Payment
	for _, p := range r.payments {
		if (f.Status == "" || p.Status == f.Status) && (f.OrderID == nil || p.OrderID == *f.OrderID) {
			all = append(all, p)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	total := len(all)
	start := min((f.Page-1)*f.PageSize, total)
	return all[start:min(start+f.PageSize, total)], total, nil
}

func (*readRepo) Begin(context.Context, *domain.Payment) (*domain.Payment, bool, error) {
	panic("the HTTP API must not write payments")
}
func (*readRepo) StartAttempt(context.Context, uuid.UUID) (*domain.Payment, error) {
	panic("the HTTP API must not write payments")
}
func (*readRepo) Complete(context.Context, uuid.UUID, domain.Outcome, messaging.Envelope) (*domain.Payment, bool, error) {
	panic("the HTTP API must not write payments")
}
func (*readRepo) Enqueue(context.Context, messaging.Envelope) error {
	panic("the HTTP API must not write payments")
}
func (*readRepo) PaidByOrder(context.Context, uuid.UUID) (*domain.Payment, error) {
	panic("the HTTP API never refunds")
}
func (*readRepo) Refund(context.Context, uuid.UUID, string, messaging.Envelope) (*domain.Payment, bool, error) {
	panic("the HTTP API never refunds")
}
func (*readRepo) HasSucceeded(context.Context, uuid.UUID) (bool, error) {
	panic("the HTTP API must not write payments")
}

type env struct {
	t    *testing.T
	app  *fiber.App
	repo *readRepo
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{t: t, repo: &readRepo{}}
	e.app = transport.NewApp(transport.Options{
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		AllowedOrigins: "https://shop.example",
		TrustedProxies: []string{"127.0.0.1"},
		Verifier:       auth.NewVerifier([]byte(testSecret)),
		InternalAPIKey: testInternalKey,
		Readiness:      map[string]httpx.ReadinessCheck{},
	}, transport.Services{Payments: application.NewQueryService(e.repo)})
	return e
}

func (e *env) seed(p domain.Payment) domain.Payment {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.Status == "" {
		p.Status = domain.StatusSucceeded
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.IdempotencyKey = "key-" + p.ID.String()
	p.AmountCents, p.Currency, p.PaymentMethod = 459900, "TRY", "tok_visa"
	if p.Status == domain.StatusSucceeded {
		p.ProviderRef = "ref_1"
	} else {
		p.FailureCode, p.FailureMessage = domain.CodeCardDeclined, "The card was declined"
	}
	e.repo.payments = append(e.repo.payments, p)
	return p
}

func tokenFor(t *testing.T, subject, role string) string {
	t.Helper()
	claims := auth.Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: auth.Issuer, Subject: subject, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type result struct {
	status int
	body   map[string]any
}

func (r result) data() map[string]any {
	d, _ := r.body["data"].(map[string]any)
	return d
}

func (r result) items() []any {
	i, _ := r.data()["items"].([]any)
	return i
}

func (e *env) get(path, bearer, internalKey string) result {
	e.t.Helper()
	return e.send("GET", path, bearer, internalKey, nil)
}

func (e *env) send(method, path, bearer, internalKey string, body any) result {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	}
	if bearer != "" {
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+bearer)
	}
	if internalKey != "" {
		req.Header.Set(transport.InternalKeyHeader, internalKey)
	}
	res, err := e.app.Test(req, 5000)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	out := result{status: res.StatusCode}
	raw, _ := io.ReadAll(res.Body)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

func TestCustomerSeesOwnPayment(t *testing.T) {
	e := newEnv(t)
	customer, order := uuid.New(), uuid.New()
	p := e.seed(domain.Payment{OrderID: order, CustomerID: customer})

	res := e.get("/api/v1/payments/by-order/"+order.String(), tokenFor(t, customer.String(), auth.RoleCustomer), "")

	if res.status != 200 {
		t.Fatalf("status %d: %v", res.status, res.body)
	}
	d := res.data()
	if d["id"] != p.ID.String() || d["status"] != "succeeded" || d["amount_cents"] != float64(459900) || d["currency"] != "TRY" {
		t.Fatalf("unexpected body: %v", d)
	}
	// The customer view must not carry operational detail.
	for _, k := range []string{"customer_id", "idempotency_key", "payment_method", "provider_ref", "attempts"} {
		if _, leaked := d[k]; leaked {
			t.Errorf("customer response leaks %q", k)
		}
	}
}

func TestCustomerGetsLatestAttemptOfAnOrder(t *testing.T) {
	e := newEnv(t)
	customer, order := uuid.New(), uuid.New()
	e.seed(domain.Payment{OrderID: order, CustomerID: customer, Status: domain.StatusFailed, CreatedAt: time.Now().Add(-time.Hour)})
	second := e.seed(domain.Payment{OrderID: order, CustomerID: customer, Status: domain.StatusSucceeded})

	res := e.get("/api/v1/payments/by-order/"+order.String(), tokenFor(t, customer.String(), auth.RoleCustomer), "")

	if res.data()["id"] != second.ID.String() || res.data()["status"] != "succeeded" {
		t.Fatalf("expected the newest attempt: %v", res.data())
	}
}

func TestFailedPaymentShowsReasonToCustomer(t *testing.T) {
	e := newEnv(t)
	customer, order := uuid.New(), uuid.New()
	e.seed(domain.Payment{OrderID: order, CustomerID: customer, Status: domain.StatusFailed})

	res := e.get("/api/v1/payments/by-order/"+order.String(), tokenFor(t, customer.String(), auth.RoleCustomer), "")

	if d := res.data(); d["status"] != "failed" || d["failure_code"] != "card_declined" || d["failure_message"] == "" {
		t.Fatalf("unexpected body: %v", d)
	}
}

func TestOtherCustomersPaymentLooksLikeItDoesNotExist(t *testing.T) {
	e := newEnv(t)
	order := uuid.New()
	e.seed(domain.Payment{OrderID: order, CustomerID: uuid.New()})
	stranger := tokenFor(t, uuid.NewString(), auth.RoleCustomer)

	existing := e.get("/api/v1/payments/by-order/"+order.String(), stranger, "")
	missing := e.get("/api/v1/payments/by-order/"+uuid.NewString(), stranger, "")

	if existing.status != 404 || missing.status != 404 {
		t.Fatalf("got %d and %d, want 404 for both", existing.status, missing.status)
	}
	if existing.body["message"] != missing.body["message"] {
		t.Fatal("the two answers must be indistinguishable")
	}
}

func TestCustomerEndpointValidation(t *testing.T) {
	e := newEnv(t)
	bearer := tokenFor(t, uuid.NewString(), auth.RoleCustomer)

	if res := e.get("/api/v1/payments/by-order/"+uuid.NewString(), "", ""); res.status != 401 {
		t.Errorf("no token = %d, want 401", res.status)
	}
	if res := e.get("/api/v1/payments/by-order/not-a-uuid", bearer, ""); res.status != 400 {
		t.Errorf("bad id = %d, want 400", res.status)
	}
	// A token whose subject is not a UUID cannot own anything.
	if res := e.get("/api/v1/payments/by-order/"+uuid.NewString(), tokenFor(t, "someone", auth.RoleCustomer), ""); res.status != 401 {
		t.Errorf("odd subject = %d, want 401", res.status)
	}
}

func TestAdminListFiltersAndPaginates(t *testing.T) {
	e := newEnv(t)
	admin := tokenFor(t, uuid.NewString(), auth.RoleAdmin)
	order := uuid.New()
	for i := 0; i < 5; i++ {
		e.seed(domain.Payment{OrderID: uuid.New(), CustomerID: uuid.New(), CreatedAt: time.Now().Add(-time.Duration(i) * time.Minute)})
	}
	e.seed(domain.Payment{OrderID: order, CustomerID: uuid.New(), Status: domain.StatusFailed})

	all := e.get("/api/v1/admin/payments", admin, "")
	if all.status != 200 || all.data()["total"] != float64(6) || len(all.items()) != 6 {
		t.Fatalf("list: %d %v", all.status, all.data())
	}
	first := all.items()[0].(map[string]any)
	for _, k := range []string{"customer_id", "idempotency_key", "payment_method", "attempts", "provider_ref"} {
		if _, ok := first[k]; !ok {
			t.Errorf("admin response lacks %q", k)
		}
	}

	failed := e.get("/api/v1/admin/payments?status=failed", admin, "")
	if failed.data()["total"] != float64(1) {
		t.Errorf("status filter: %v", failed.data())
	}
	byOrder := e.get("/api/v1/admin/payments?order_id="+order.String(), admin, "")
	if byOrder.data()["total"] != float64(1) {
		t.Errorf("order filter: %v", byOrder.data())
	}
	page := e.get("/api/v1/admin/payments?page=2&page_size=4", admin, "")
	if len(page.items()) != 2 || page.data()["total"] != float64(6) {
		t.Errorf("pagination: %v", page.data())
	}

	for _, q := range []string{"?status=bogus", "?order_id=nope"} {
		if res := e.get("/api/v1/admin/payments"+q, admin, ""); res.status != 400 {
			t.Errorf("%s = %d, want 400", q, res.status)
		}
	}
}

func TestAdminGetByID(t *testing.T) {
	e := newEnv(t)
	admin := tokenFor(t, uuid.NewString(), auth.RoleAdmin)
	p := e.seed(domain.Payment{OrderID: uuid.New(), CustomerID: uuid.New()})

	if res := e.get("/api/v1/admin/payments/"+p.ID.String(), admin, ""); res.status != 200 || res.data()["id"] != p.ID.String() {
		t.Errorf("get: %d %v", res.status, res.body)
	}
	if res := e.get("/api/v1/admin/payments/"+uuid.NewString(), admin, ""); res.status != 404 {
		t.Errorf("missing = %d, want 404", res.status)
	}
	if res := e.get("/api/v1/admin/payments/zzz", admin, ""); res.status != 400 {
		t.Errorf("bad id = %d, want 400", res.status)
	}
}

func TestAdminEndpointsRequireAdminRole(t *testing.T) {
	e := newEnv(t)
	p := e.seed(domain.Payment{OrderID: uuid.New(), CustomerID: uuid.New()})
	customer := tokenFor(t, uuid.NewString(), auth.RoleCustomer)

	for _, path := range []string{"/api/v1/admin/payments", "/api/v1/admin/payments/" + p.ID.String()} {
		if res := e.get(path, "", ""); res.status != 401 {
			t.Errorf("%s without token = %d, want 401", path, res.status)
		}
		if res := e.get(path, customer, ""); res.status != 403 {
			t.Errorf("%s as customer = %d, want 403", path, res.status)
		}
		// The service-to-service key is not a user credential.
		if res := e.get(path, "", testInternalKey); res.status != 401 {
			t.Errorf("%s with internal key = %d, want 401", path, res.status)
		}
	}
}

func TestInternalLookupNeedsTheServiceKey(t *testing.T) {
	e := newEnv(t)
	order := uuid.New()
	e.seed(domain.Payment{OrderID: order, CustomerID: uuid.New()})
	path := "/api/v1/internal/payments/by-order/" + order.String()

	if res := e.get(path, "", ""); res.status != 401 {
		t.Errorf("no key = %d, want 401", res.status)
	}
	if res := e.get(path, "", testInternalKey+"x"); res.status != 401 {
		t.Errorf("wrong key = %d, want 401", res.status)
	}
	// A valid user token (even an admin's) does not replace the key.
	if res := e.get(path, tokenFor(t, uuid.NewString(), auth.RoleAdmin), ""); res.status != 401 {
		t.Errorf("admin token = %d, want 401", res.status)
	}
	res := e.get(path, "", testInternalKey)
	if res.status != 200 || res.data()["order_id"] != order.String() {
		t.Errorf("valid key = %d %v", res.status, res.body)
	}
	if res := e.get("/api/v1/internal/payments/by-order/"+uuid.NewString(), "", testInternalKey); res.status != 404 {
		t.Errorf("unknown order = %d, want 404", res.status)
	}
}

func TestThereIsNoWayToCreateOrChangePaymentsOverHTTP(t *testing.T) {
	e := newEnv(t)
	admin := tokenFor(t, uuid.NewString(), auth.RoleAdmin)
	id := uuid.NewString()
	for _, r := range []struct{ method, path string }{
		{"POST", "/api/v1/payments"},
		{"POST", "/api/v1/admin/payments"},
		{"PUT", "/api/v1/admin/payments/" + id},
		{"PATCH", "/api/v1/admin/payments/" + id},
		{"DELETE", "/api/v1/admin/payments/" + id},
		{"POST", "/api/v1/internal/payments"},
		{"POST", "/api/v1/payments/by-order/" + id},
	} {
		res := e.send(r.method, r.path, admin, testInternalKey, map[string]any{"amount_cents": 1})
		if res.status != 404 && res.status != 405 {
			t.Errorf("%s %s = %d, want 404 or 405", r.method, r.path, res.status)
		}
	}
}

func TestAlgNoneTokenIsRejected(t *testing.T) {
	e := newEnv(t)
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	forged := enc(`{"alg":"none","typ":"JWT"}`) + "." +
		enc(`{"role":"admin","iss":"`+auth.Issuer+`","sub":"`+uuid.NewString()+`","exp":`+"9999999999"+`}`) + "."

	if res := e.get("/api/v1/admin/payments", forged, ""); res.status != 401 {
		t.Errorf("alg=none token = %d, want 401", res.status)
	}
}
