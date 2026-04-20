package httpserver

import (
	"net/http"
)

// registerRoutes wires all application routes onto mux.
// Handlers are thin: they decode, validate, call a service, and respond.
// Route handlers for each resource will be registered here as phases progress.
func registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", handleHealthz)
}
