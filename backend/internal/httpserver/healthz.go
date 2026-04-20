package httpserver

import (
	"net/http"

	"github.com/johannesniedens/towerdefense/internal/httpserver/respond"
)

// buildVersion is overridden at link time via
//
//	-ldflags "-X github.com/johannesniedens/towerdefense/internal/httpserver.buildVersion=<sha>"
//
// Falls back to "dev" when running locally without flags.
var buildVersion = "dev"

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": buildVersion,
	})
}
