package kate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/stn81/govalidator"
	"github.com/stn81/kate/log/ctxzap"
	"github.com/stn81/kate/utils"
	"go.uber.org/zap"
)

const (
	// HeaderContentType the header name of `Content-Type`
	HeaderContentType = "Content-Type"
	// MIMEApplicationJSON the application type for json
	MIMEApplicationJSON = "application/json"
	// MIMEApplicationJSONCharsetUTF8 the application type for json of utf-8 encoding
	MIMEApplicationJSONCharsetUTF8 = "application/json; charset=UTF-8"
)

// BaseHandler is the enhanced version of ngs.BaseController
type BaseHandler struct{}

// ParseRequest parses and validates the api request
func (h *BaseHandler) ParseRequest(ctx context.Context, r *Request, req any) error {
	logger := ctxzap.Extract(ctx)

	// decode json
	if r.ContentLength != 0 {
		if err := h.parseBody(req, r); err != nil {
			logger.Error("decode request", zap.Error(err))
			return ErrBadParam(err)
		}
	}

	// decode query
	queryValues := r.URL.Query()
	if len(queryValues) > 0 {
		data := make(map[string]any)
		for key := range queryValues {
			if value := queryValues.Get(key); len(value) > 0 {
				data[key] = value
			}
		}

		if err := utils.Bind(req, "query", data); err != nil {
			logger.Error("bind query var failed", zap.Error(err))
			return ErrBadParam(err)
		}
	}

	// decode rest var
	if len(r.RestVars) > 0 {
		data := make(map[string]any)
		for i := range r.RestVars {
			data[r.RestVars[i].Key] = r.RestVars[i].Value
		}

		if err := utils.Bind(req, "rest", data); err != nil {
			logger.Error("bind rest var failed", zap.Error(err))
			return ErrBadParam(err)
		}
	}

	// set defaults
	if err := utils.SetDefaults(req); err != nil {
		logger.Error("set default failed", zap.Error(err))
		return ErrServerInternal
	}
	// validate
	if err := govalidator.ValidateStruct(req); err != nil {
		logger.Error("validate request", zap.Error(err))
		return ErrBadParam(err)
	}
	return nil
}

// Error writes out an error response
func (h *BaseHandler) Error(ctx context.Context, w http.ResponseWriter, err error) {
	var errInfo ErrorInfo
	if !errors.As(err, &errInfo) {
		errInfo = ErrServerInternal
	}

	result := &Result{
		ErrNO:  errInfo.Code(),
		ErrMsg: errInfo.Error(),
	}

	var errInfoWithData ErrorInfoWithData
	if errors.As(errInfo, &errInfoWithData) {
		result.Data = errInfoWithData.Data()
	}

	if err := h.WriteJSON(w, result); err != nil {
		ctxzap.Extract(ctx).Error("write json response", zap.Error(err))
	}
}

// OK writes out a success response without data, used typically in an `update` api.
func (h *BaseHandler) OK(ctx context.Context, w http.ResponseWriter) {
	h.OKData(ctx, w, nil)
}

// OKData writes out a success response with data, used typically in an `get` api.
func (h *BaseHandler) OKData(ctx context.Context, w http.ResponseWriter, data any) {
	result := &Result{
		ErrNO:  ErrSuccess.Code(),
		ErrMsg: ErrSuccess.Error(),
		Data:   data,
	}

	if err := h.WriteJSON(w, result); err != nil {
		ctxzap.Extract(ctx).Error("write json response", zap.Error(err))
	}
}

// EncodeJSON is a wrapper of json.Marshal()
func (h *BaseHandler) EncodeJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// WriteJSON writes out an object which is serialized as json.
func (h *BaseHandler) WriteJSON(w http.ResponseWriter, v any) error {
	b, err := h.EncodeJSON(v)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", MIMEApplicationJSONCharsetUTF8)
	if _, err = w.Write(b); err != nil {
		return err
	}
	return nil
}

// parseBody 从http request 中解出json body，必须是 application/json
func (h *BaseHandler) parseBody(ptr any, req *Request) (err error) {
	ctype := req.Header.Get(HeaderContentType)
	switch {
	case strings.HasPrefix(ctype, MIMEApplicationJSON):
		if err = utils.ParseJSON(req.RawBody, ptr); err != nil {
			var (
				ute *json.UnmarshalTypeError
				se  *json.SyntaxError
			)
			if errors.As(err, &ute) {
				return fmt.Errorf("unmarshal type error: expected=%v, got=%v, offset=%v",
					ute.Type, ute.Value, ute.Offset)
			} else if errors.As(err, &se) {
				return fmt.Errorf("syntax error: offset=%v, error=%v",
					se.Offset, se.Error())
			} else {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported media type")
	}
	return nil
}
