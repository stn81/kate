package kate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func decodeResult(t *testing.T, body []byte) *Result {
	t.Helper()
	r := &Result{}
	if err := json.Unmarshal(body, r); err != nil {
		t.Fatalf("body is not a Result envelope: %v: %s", err, body)
	}
	return r
}

func TestRESTHandler_ErrorWithCarrierStatus(t *testing.T) {
	h := &RESTHandler{}
	rec := httptest.NewRecorder()
	err := NewHTTPError(http.StatusServiceUnavailable, 503, "bytehouse down")

	h.Error(context.Background(), rec, err)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != MIMEApplicationJSONCharsetUTF8 {
		t.Errorf("Content-Type = %q（必须在 WriteHeader 之前设置）", ct)
	}
	got := decodeResult(t, rec.Body.Bytes())
	if got.ErrNO != 503 || got.ErrMsg != "bytehouse down" {
		t.Errorf("envelope wrong: %+v", got)
	}

	// envelope 与 BaseHandler 对同一错误的输出字节级一致（双风格只差状态码）
	recBase := httptest.NewRecorder()
	(&BaseHandler{}).Error(context.Background(), recBase, err)
	if rec.Body.String() != recBase.Body.String() {
		t.Errorf("REST body %q != errno-style body %q", rec.Body.String(), recBase.Body.String())
	}
}

func TestRESTHandler_ErrorBadParamMapsTo400(t *testing.T) {
	h := &RESTHandler{}
	rec := httptest.NewRecorder()

	h.Error(context.Background(), rec, ErrBadParam("date is required"))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	got := decodeResult(t, rec.Body.Bytes())
	if got.ErrNO != errnoBadParam || got.ErrMsg != "date is required" {
		t.Errorf("envelope wrong: %+v", got)
	}
}

func TestRESTHandler_ErrorPlainErrorMapsTo500(t *testing.T) {
	h := &RESTHandler{}
	rec := httptest.NewRecorder()

	h.Error(context.Background(), rec, errors.New("boom"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	got := decodeResult(t, rec.Body.Bytes())
	if got.ErrNO != ErrServerInternal.Code() || got.ErrMsg != ErrServerInternal.Error() {
		t.Errorf("plain error should render as ErrServerInternal: %+v", got)
	}
}

func TestRESTHandler_ErrorPreservesData(t *testing.T) {
	h := &RESTHandler{}
	rec := httptest.NewRecorder()
	err := WithHTTPStatus(NewErrorWithData(10004, "用户不存在", map[string]any{"id": float64(1)}), http.StatusNotFound)

	h.Error(context.Background(), rec, err)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	got := decodeResult(t, rec.Body.Bytes())
	m, ok := got.Data.(map[string]any)
	if got.ErrNO != 10004 || !ok || m["id"] != float64(1) {
		t.Errorf("data lost in envelope: %+v", got)
	}
}

func TestRESTHandler_OkDataStays200(t *testing.T) {
	h := &RESTHandler{}
	rec := httptest.NewRecorder()

	h.OkData(context.Background(), rec, map[string]string{"status": "ok"})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	got := decodeResult(t, rec.Body.Bytes())
	if got.ErrNO != ErrSuccess.Code() {
		t.Errorf("success envelope wrong: %+v", got)
	}
}

// 回归：BaseHandler 的契约（恒 200）不被带 HTTPStatus 的错误值穿透。
func TestBaseHandler_ErrorIgnoresCarrierStatus(t *testing.T) {
	h := &BaseHandler{}
	rec := httptest.NewRecorder()
	err := NewHTTPError(http.StatusServiceUnavailable, 503, "x")

	h.Error(context.Background(), rec, err)

	if rec.Code != http.StatusOK {
		t.Errorf("errno-style must stay 200 even for carrier errors, got %d", rec.Code)
	}
	got := decodeResult(t, rec.Body.Bytes())
	if got.ErrNO != 503 {
		t.Errorf("errno should still surface in body: %+v", got)
	}
}

// 经 kate 自己的 responseWriter 管线（中间件看到的 StatusCode 口径）。
func TestRESTHandler_ErrorThroughKateResponseWriter(t *testing.T) {
	h := &RESTHandler{}
	rec := httptest.NewRecorder()
	w := &responseWriter{ResponseWriter: rec}

	h.Error(context.Background(), w, NewHTTPError(http.StatusTooManyRequests, 429, "slow down"))

	if w.StatusCode() != http.StatusTooManyRequests {
		t.Errorf("responseWriter.StatusCode() = %d, want 429", w.StatusCode())
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("underlying recorder status = %d, want 429", rec.Code)
	}
}
