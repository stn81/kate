package kate

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewHTTPError(t *testing.T) {
	e := NewHTTPError(http.StatusServiceUnavailable, 503, "bytehouse down")

	if e.Code() != 503 {
		t.Errorf("Code() = %d, want 503", e.Code())
	}
	if e.Error() != "bytehouse down" {
		t.Errorf("Error() = %q, want %q", e.Error(), "bytehouse down")
	}
	var c HTTPStatusCarrier
	if !errors.As(e, &c) {
		t.Fatal("NewHTTPError should carry HTTPStatusCarrier")
	}
	if c.HTTPStatus() != http.StatusServiceUnavailable {
		t.Errorf("HTTPStatus() = %d, want 503", c.HTTPStatus())
	}
}

func TestWithHTTPStatus_PreservesCodeMessageData(t *testing.T) {
	base := NewErrorWithData(10004, "用户不存在", map[string]any{"id": 1})
	e := WithHTTPStatus(base, http.StatusNotFound)

	// ErrorInfo 语义不变
	if e.Code() != 10004 || e.Error() != "用户不存在" {
		t.Errorf("wrapped error changed identity: code=%d msg=%q", e.Code(), e.Error())
	}
	// 状态可取
	var c HTTPStatusCarrier
	if !errors.As(e, &c) || c.HTTPStatus() != http.StatusNotFound {
		t.Fatalf("HTTPStatus not carried: %v", e)
	}
	// Data 经 errors.As 链可达（BaseHandler/RESTHandler 的取法）
	var wd ErrorInfoWithData
	if !errors.As(e, &wd) {
		t.Fatal("ErrorInfoWithData lost after WithHTTPStatus")
	}
	if m, ok := wd.Data().(map[string]any); !ok || m["id"] != 1 {
		t.Errorf("Data() lost: %v", wd.Data())
	}
}

func TestWithHTTPStatus_PlainErrorInfo(t *testing.T) {
	e := WithHTTPStatus(NewError(20001, "余额不足"), http.StatusConflict)
	var c HTTPStatusCarrier
	if !errors.As(e, &c) || c.HTTPStatus() != http.StatusConflict {
		t.Fatal("HTTPStatus not carried for plain ErrorInfo")
	}
	// 无 Data 的错误不能凭空多出 ErrorInfoWithData
	var wd ErrorInfoWithData
	if errors.As(e, &wd) {
		t.Error("plain ErrorInfo must not gain ErrorInfoWithData after wrapping")
	}
}
