package kate

import (
	"context"
	"github.com/stn81/kate/middleware"
	"net/http"

	"github.com/stn81/kate/log/ctxzap"
	"go.uber.org/zap"
)

// Router defines the standard http outer
type Router struct {
	*http.ServeMux
	maxBodyBytes int64
	ctx          context.Context
}

// NewRouter create a http router
func NewRouter(ctx context.Context, logger *zap.Logger) *Router {
	return &Router{
		ServeMux: http.NewServeMux(),
		ctx:      ctxzap.ToContext(ctx, logger),
	}
}

// SetMaxBodyBytes set the body size limit
func (r *Router) SetMaxBodyBytes(n int64) {
	r.maxBodyBytes = n
}

// StdHandle register a standard http handler for the specified path
func (r *Router) StdHandle(pattern string, h http.Handler) {
	r.ServeMux.Handle(pattern, h)
}

// Handle register a http handler for the specified path
func (r *Router) Handle(pattern string, h ContextHandler) {
	r.ServeMux.Handle(pattern, StdHandler(r.ctx, h, r.maxBodyBytes))
}

// HandleFunc register a http handler for the specified path
func (r *Router) HandleFunc(pattern string, h func(context.Context, ResponseWriter, *Request)) {
	r.Handle(pattern, ContextHandlerFunc(h))
}

// HEAD register a handler for HEAD request
func (r *Router) HEAD(pattern string, h ContextHandler) {
	r.ServeMux.Handle(pattern, StdHandler(r.ctx, middleware.HEAD(h), r.maxBodyBytes))
}

// OPTIONS register a handler for OPTIONS request
func (r *Router) OPTIONS(pattern string, h ContextHandler) {
	r.ServeMux.Handle(pattern, StdHandler(r.ctx, middleware.OPTIONS(h), r.maxBodyBytes))
}

// GET register a handler for GET request
func (r *Router) GET(pattern string, h ContextHandler) {
	r.ServeMux.Handle(pattern, StdHandler(r.ctx, middleware.GET(h), r.maxBodyBytes))
}

// POST register a handler for POST request
func (r *Router) POST(pattern string, h ContextHandler) {
	r.ServeMux.Handle(pattern, StdHandler(r.ctx, middleware.POST(h), r.maxBodyBytes))
}

// PUT register a handler for PUT request
func (r *Router) PUT(pattern string, h ContextHandler) {
	r.ServeMux.Handle(pattern, StdHandler(r.ctx, middleware.PUT(h), r.maxBodyBytes))
}

// DELETE register a handler for DELETE request
func (r *Router) DELETE(pattern string, h ContextHandler) {
	r.ServeMux.Handle(pattern, StdHandler(r.ctx, middleware.DELETE(h), r.maxBodyBytes))
}

// PATCH register a handler for PATCH request
func (r *Router) PATCH(pattern string, h ContextHandler) {
	r.ServeMux.Handle(pattern, StdHandler(r.ctx, middleware.PATCH(h), r.maxBodyBytes))
}
