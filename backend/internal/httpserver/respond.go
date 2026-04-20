package httpserver

import (
	"net/http"

	"github.com/johannesniedens/towerdefense/internal/httpserver/respond"
)

// RespondJSON delegates to respond.JSON. Kept for callers outside the
// handlers sub-package that expect the httpserver-scoped name.
func RespondJSON(w http.ResponseWriter, status int, v any) {
	respond.JSON(w, status, v)
}

// RespondError delegates to respond.Error. Kept for callers outside the
// handlers sub-package that expect the httpserver-scoped name.
func RespondError(w http.ResponseWriter, r *http.Request, err error) {
	respond.Error(w, r, err)
}
