package httpx

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Furkanturan8/rovena/shared/telemetry"
	"github.com/gofiber/fiber/v2"
)

var setupOnce sync.Once

func tracedApp(t *testing.T) (*fiber.App, *string) {
	t.Helper()
	setupOnce.Do(func() {
		if _, err := telemetry.Setup(context.Background(), "httpx-test", nil); err != nil {
			t.Fatal(err)
		}
	})
	var seen string
	app := fiber.New()
	app.Use(Tracing())
	app.Get("/items/:id", func(c *fiber.Ctx) error {
		seen = telemetry.TraceID(c.UserContext())
		return c.SendString("ok")
	})
	app.Get("/boom", func(c *fiber.Ctx) error { return fiber.NewError(500, "boom") })
	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })
	RegisterMetrics(app)
	return app, &seen
}

func scrape(t *testing.T, app *fiber.App) string {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", "/metrics", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestIncomingTraceContinuesAndIsReportedBack(t *testing.T) {
	app, seen := tracedApp(t)
	const trace = "4bf92f3577b34da6a3ce929d0e0e4736"
	req := httptest.NewRequest("GET", "/items/7", nil)
	req.Header.Set("traceparent", "00-"+trace+"-00f067aa0ba902b7-01")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if *seen != trace {
		t.Fatalf("the handler runs in trace %q, want the caller's %q", *seen, trace)
	}
	if got := resp.Header.Get("X-Trace-ID"); got != trace {
		t.Fatalf("X-Trace-ID = %q", got)
	}
}

func TestRequestWithoutTraceparentStartsANewTrace(t *testing.T) {
	app, seen := tracedApp(t)
	resp, _ := app.Test(httptest.NewRequest("GET", "/items/7", nil))
	defer func() { _ = resp.Body.Close() }()
	if len(*seen) != 32 || resp.Header.Get("X-Trace-ID") != *seen {
		t.Fatalf("trace %q, header %q", *seen, resp.Header.Get("X-Trace-ID"))
	}
}

func TestMetricsUseTheRouteTemplateNotTheURL(t *testing.T) {
	app, _ := tracedApp(t)
	for _, id := range []string{"1", "2", "3"} {
		do(app, httptest.NewRequest("GET", "/items/"+id, nil))
	}
	do(app, httptest.NewRequest("GET", "/boom", nil))
	do(app, httptest.NewRequest("GET", "/healthz", nil))
	do(app, httptest.NewRequest("GET", "/no/such/path/42", nil))

	out := scrape(t, app)
	for _, want := range []string{
		`http_server_request_duration_seconds_count{`,
		`http_route="/items/:id"`,
		`http_response_status_code="500"`,
		`http_route="unmatched"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics lack %s", want)
		}
	}
	// One time series per route, not one per id (that would never stop growing).
	if strings.Contains(out, `/items/1`) || strings.Contains(out, `/no/such/path`) {
		t.Error("raw URLs leaked into metric labels")
	}
	if strings.Contains(out, `http_route="/healthz"`) || strings.Contains(out, `http_route="/metrics"`) {
		t.Error("probe and scrape requests must not be counted")
	}
}

// Fiber's strings point into buffers reused by the next request on the same connection. Over a
// real keep-alive connection a label that was not copied turns into "GETT" or "PUTETE".
func TestLabelsSurviveConnectionReuse(t *testing.T) {
	app, _ := tracedApp(t)
	app.Put("/items/:id", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Delete("/items/:id", func(c *fiber.Ctx) error { return c.SendStatus(204) })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(ln) }()
	client := &http.Client{}
	// Shutdown waits for open connections; the client's idle keep-alive one would hold it up.
	t.Cleanup(func() { client.CloseIdleConnections(); _ = app.ShutdownWithTimeout(2 * time.Second) })

	for _, m := range []string{"GET", "PUT", "DELETE", "GET", "DELETE", "PUT"} {
		req, _ := http.NewRequest(m, "http://"+ln.Addr().String()+"/items/9", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	out := scrape(t, app)
	for _, bad := range []string{`GETT`, `PUTETE`, `DELETEE`, `GETETE`} {
		if strings.Contains(out, `"`+bad+`"`) {
			t.Fatalf("corrupted method label %s in:\n%s", bad, out)
		}
	}
	for _, m := range []string{"GET", "PUT", "DELETE"} {
		if !strings.Contains(out, `http_request_method="`+m+`"`) {
			t.Errorf("no series for %s", m)
		}
	}
}

// do sends a request and discards the answer.
func do(app *fiber.App, req *http.Request) {
	if resp, err := app.Test(req); err == nil {
		_ = resp.Body.Close()
	}
}
