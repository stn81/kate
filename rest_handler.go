package kate

import (
	"context"
	"errors"
	"net/http"

	"github.com/stn81/kate/log"
	"go.uber.org/zap"
)

// RESTHandler 是 HTTP 状态码风格的 handler 基类：错误用真实状态码表达（4xx/5xx），
// body 仍是与 BaseHandler 完全一致的 {errno, errmsg, data?} envelope——双风格只差
// 状态码这一个维度，客户端解析逻辑通用。
//
// 风格是端点契约：嵌入 RESTHandler 即声明本端点走状态码风格；嵌入 BaseHandler 则
// 恒 200（即使错误值带 HTTPStatus 也不穿透）。同一个 service 层错误在两种端点下
// 各按契约渲染。
//
// 状态码取值优先级：
//  1. 错误实现 HTTPStatusCarrier（NewHTTPError / WithHTTPStatus）→ 显式状态
//  2. 框架内置错误：ErrBadParam → 400
//  3. 其余（含非 ErrorInfo 的裸 error）→ 500
type RESTHandler struct {
	BaseHandler
}

// Error writes the error response with a real http status code.
func (h *RESTHandler) Error(ctx context.Context, w http.ResponseWriter, err error) {
	errInfo, result := errorResult(err)

	b, jerr := h.EncodeJson(result)
	if jerr != nil {
		log.GetLogger(ctx).Error("encode json response", zap.Error(jerr))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// header 必须在 WriteHeader 之前设置，否则丢失。
	w.Header().Set(HeaderContentType, MIMEApplicationJSONCharsetUTF8)
	w.WriteHeader(httpStatusOf(errInfo))
	if _, werr := w.Write(b); werr != nil {
		log.GetLogger(ctx).Error("write json response", zap.Error(werr))
	}
}

// httpStatusOf 解析错误应使用的 HTTP 状态码（见 RESTHandler 注释的优先级）。
func httpStatusOf(errInfo ErrorInfo) int {
	var carrier HTTPStatusCarrier
	if errors.As(errInfo, &carrier) {
		return carrier.HTTPStatus()
	}
	switch errInfo.Code() {
	case errnoBadParam:
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
