package middleware

import (
	"context"
	"github.com/stn81/kate"
)

// Chain is the middleware chain
type Chain struct {
	middlewares []Middleware
}

// NewChain create a new middleware chain
func NewChain(middlewares ...Middleware) Chain {
	c := Chain{}
	c.middlewares = append(c.middlewares, middlewares...)

	return c
}

// Then return a handler wrapped by the middleware chain
func (c Chain) Then(h kate.ContextHandler) kate.ContextHandler {
	if h == nil {
		panic("handler == nil")
	}

	final := h

	for i := len(c.middlewares) - 1; i >= 0; i-- {
		final = c.middlewares[i](final)
	}

	return final
}

// ThenFunc return a handler wrapped by the middleware chain
func (c Chain) ThenFunc(h func(context.Context, kate.ResponseWriter, *kate.Request)) kate.ContextHandler {
	return c.Then(kate.ContextHandlerFunc(h))
}

// Append return a new middleware chain with new middleware appended
func (c Chain) Append(middlewares ...Middleware) Chain {
	newMws := make([]Middleware, len(c.middlewares)+len(middlewares))
	copy(newMws, c.middlewares)
	copy(newMws[len(c.middlewares):], middlewares)

	newChain := NewChain(newMws...)
	return newChain
}
