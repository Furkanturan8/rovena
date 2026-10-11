// Package clients holds the REST clients Order uses to call Catalog and Inventory.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Furkanturan8/rovena/shared/messaging"
)

const internalKeyHeader = "X-Internal-Key"

// maxResponseBytes bounds what is read from a peer; none of the answers come close.
const maxResponseBytes = 1 << 20

type base struct {
	url string
	key string
	hc  *http.Client
}

func newBase(url, internalKey string, timeout time.Duration) base {
	return base{url: url, key: internalKey, hc: &http.Client{Timeout: timeout}}
}

// do sends a JSON request and returns the status and body. A transport error (timeout,
// connection refused) is returned as an error; any HTTP status is returned as is.
func (b base) do(ctx context.Context, method, path string, body any) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.url+path, rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set(internalKeyHeader, b.key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if id := messaging.RequestID(ctx); id != "" {
		req.Header.Set("X-Request-ID", id)
	}
	resp, err := b.hc.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, raw, nil
}

func unexpected(service string, status int, body []byte) error {
	if len(body) > 200 {
		body = body[:200]
	}
	return fmt.Errorf("%s answered %d: %s", service, status, body)
}
