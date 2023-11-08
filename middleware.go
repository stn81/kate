package kate

// MiddlewareFunc defines the middleware func
type MiddlewareFunc func(ContextHandler) ContextHandler

func (f MiddlewareFunc) Proxy(h ContextHandler) ContextHandler {
	return f(h)
}

// Middleware defines the middleware interface
type Middleware interface {
	Proxy(handler ContextHandler) ContextHandler
}
