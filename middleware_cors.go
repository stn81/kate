package kate

import (
	"context"
	"strconv"
)

func CORS(maxAge int) Middleware {
	mf := func(h ContextHandler) ContextHandler {
		f := func(ctx context.Context, w ResponseWriter, r *Request) {
			corsWriter := &corsResponseWriter{
				ResponseWriter: w,
				maxAge:         maxAge,
			}
			h.ServeHTTP(ctx, corsWriter, r)
		}
		return ContextHandlerFunc(f)
	}
	return MiddlewareFunc(mf)
}

type corsResponseWriter struct {
	ResponseWriter
	maxAge int
}

func (w *corsResponseWriter) WriteHeader(statusCode int) {
	w.setCORSHeaders()
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *corsResponseWriter) Write(b []byte) (int, error) {
	w.setCORSHeaders()
	return w.ResponseWriter.Write(b)
}

func (w *corsResponseWriter) setCORSHeaders() {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", h.Get("Allow"))
	h.Set("Access-Control-Allow-Headers", "*")
	h.Set("Access-Control-Allow-Credentials", "true")
	h.Set("Access-Control-Max-Age", strconv.Itoa(w.maxAge))
}
