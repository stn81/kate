package middleware

import "github.com/stn81/kate"

// Middleware defines the middleware func
type Middleware func(kate.ContextHandler) kate.ContextHandler
