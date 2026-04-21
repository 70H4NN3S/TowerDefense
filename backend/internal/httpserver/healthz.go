package httpserver

import (
	"net/http"

	"github.com/70H4NN3S/TowerDefense/internal/httpserver/respond"
)

// buildVersion is overridden at link time via
//
//	-ldflags "-X github.com/70H4NN3S/TowerDefense/internal/httpserver.buildVersion=<sha>"
//
// Falls back to "dev" when running locally without flags.
var buildVersion = "dev"

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": buildVersion,
	})
}
