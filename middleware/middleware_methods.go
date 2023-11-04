package middleware

import (
	"context"
	"github.com/stn81/kate"
	"net/http"
	"strings"
)

// MethodOnly is a middleware to restrict http method for standard http router
func MethodOnly(method string, h kate.ContextHandler) kate.ContextHandler {
	f := func(ctx context.Context, w kate.ResponseWriter, r *kate.Request) {
		if strings.ToUpper(r.Method) != method {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte(http.StatusText(http.StatusMethodNotAllowed)))
			return
		}

		h.ServeHTTP(ctx, w, r)
	}
	return kate.ContextHandlerFunc(f)
}

// HEAD only allow HEAD method
func HEAD(h kate.ContextHandler) kate.ContextHandler {
	return MethodOnly("HEAD", h)
}

// OPTIONS only allow OPTIONS method
func OPTIONS(h kate.ContextHandler) kate.ContextHandler {
	return MethodOnly("OPTIONS", h)
}

// GET only allow GET method
func GET(h kate.ContextHandler) kate.ContextHandler {
	return MethodOnly("GET", h)
}

// POST only allow POST method
func POST(h kate.ContextHandler) kate.ContextHandler {
	return MethodOnly("POST", h)
}

// PUT only allow PUT method
func PUT(h kate.ContextHandler) kate.ContextHandler {
	return MethodOnly("PUT", h)
}

// DELETE only allow DELETE method
func DELETE(h kate.ContextHandler) kate.ContextHandler {
	return MethodOnly("DELETE", h)
}

// PATCH only allow PATCH method
func PATCH(h kate.ContextHandler) kate.ContextHandler {
	return MethodOnly("PATCH", h)
}
