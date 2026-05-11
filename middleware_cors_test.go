package kate
package kate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	request := &Request{Request: req}
	response := &responseWriter{ResponseWriter: recorder}

	handler := ContextHandlerFunc(func(ctx context.Context, w ResponseWriter, r *Request) {
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	})

	CORS(600).Proxy(handler).ServeHTTP(context.Background(), response, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin=* got=%q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("expected Access-Control-Allow-Methods=GET, POST, OPTIONS got=%q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Headers=* got=%q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected Access-Control-Allow-Credentials=true got=%q", got)
	}
	if got := recorder.Header().Get("Access-Control-Max-Age"); got != "600" {
		t.Fatalf("expected Access-Control-Max-Age=600 got=%q", got)
	}
}
