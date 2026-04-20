// Package middleware provides HTTP middleware constructors for the Tower Defense
// server. All middleware follow the standard func(http.Handler) http.Handler
// signature so they compose cleanly with Chain.
package middleware

import "net/http"

// Middleware is a function that wraps an http.Handler.
type Middleware = func(http.Handler) http.Handler

// Chain applies middlewares to h in declaration order: the first middleware in
// the slice is the outermost wrapper and therefore the first to execute on each
// request.
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
