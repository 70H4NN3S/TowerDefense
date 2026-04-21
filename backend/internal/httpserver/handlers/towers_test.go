package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/70H4NN3S/TowerDefense/internal/game"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fakeTowerService ──────────────────────────────────────────────────────────

type fakeTowerService struct {
	owned      map[string]game.OwnedTower // key: "userID:templateID"
	levelUpErr error
}

func newFakeTowerService() *fakeTowerService {
	return &fakeTowerService{
		owned: make(map[string]game.OwnedTower),
	}
}

func (f *fakeTowerService) towerOwnedKey(userID, templateID uuid.UUID) string {
	return userID.String() + ":" + templateID.String()
}

func (f *fakeTowerService) seed(o game.OwnedTower) {
	f.owned[f.towerOwnedKey(o.UserID, o.TemplateID)] = o
}

func (f *fakeTowerService) ListOwned(_ context.Context, userID uuid.UUID) ([]game.OwnedTower, error) {
	var out []game.OwnedTower
	for _, o := range f.owned {
		if o.UserID == userID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeTowerService) LevelUp(_ context.Context, userID, templateID uuid.UUID) (game.OwnedTower, error) {
	if f.levelUpErr != nil {
		return game.OwnedTower{}, f.levelUpErr
	}
	key := f.towerOwnedKey(userID, templateID)
	o, ok := f.owned[key]
	if !ok {
		return game.OwnedTower{}, game.ErrNotOwned
	}
	o.Level++
	f.owned[key] = o
	return o, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newTowerMux(svc *fakeTowerService) *http.ServeMux {
	mux := http.NewServeMux()
	NewTowerHandler(svc, testSecret).Register(mux)
	return mux
}

// ── GET /v1/towers ────────────────────────────────────────────────────────────

func TestListOwned_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newTowerMux(newFakeTowerService())
	w := doRequest(mux, http.MethodGet, "/v1/towers", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestListOwned_EmptyReturns200(t *testing.T) {
	t.Parallel()
	mux := newTowerMux(newFakeTowerService())
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, "/v1/towers", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	type towersResp struct {
		Towers []ownedTowerResponse `json:"towers"`
	}
	resp := decodeResponse[towersResp](t, w)
	if len(resp.Towers) != 0 {
		t.Errorf("got %d towers, want 0", len(resp.Towers))
	}
}

func TestListOwned_ReturnsTowers(t *testing.T) {
	t.Parallel()
	svc := newFakeTowerService()
	userID := uuid.New()
	svc.seed(game.OwnedTower{
		UserID:     userID,
		TemplateID: shopTemplateID,
		Level:      3,
		Template:   shopTemplate,
		Current:    game.TowerLevel{Level: 3, Damage: 30, Range: 150, Rate: 12},
	})

	mux := newTowerMux(svc)
	tok := signedToken(t, userID)
	w := doRequest(mux, http.MethodGet, "/v1/towers", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	type towersResp struct {
		Towers []ownedTowerResponse `json:"towers"`
	}
	resp := decodeResponse[towersResp](t, w)
	if len(resp.Towers) != 1 {
		t.Fatalf("got %d towers, want 1", len(resp.Towers))
	}
	if resp.Towers[0].Current.Level != 3 {
		t.Errorf("level = %d, want 3", resp.Towers[0].Current.Level)
	}
}

// ── POST /v1/towers/{id}/upgrade ─────────────────────────────────────────────

func TestUpgradeTower_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newTowerMux(newFakeTowerService())
	w := doRequest(mux, http.MethodPost, "/v1/towers/"+shopTemplateID.String()+"/upgrade", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestUpgradeTower_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()
	mux := newTowerMux(newFakeTowerService())
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/towers/bad-uuid/upgrade", tok, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestUpgradeTower_HappyPath_Returns200(t *testing.T) {
	t.Parallel()
	svc := newFakeTowerService()
	userID := uuid.New()
	svc.seed(game.OwnedTower{
		UserID:     userID,
		TemplateID: shopTemplateID,
		Level:      1,
		Template:   shopTemplate,
	})
	mux := newTowerMux(svc)
	tok := signedToken(t, userID)

	w := doRequest(mux, http.MethodPost, "/v1/towers/"+shopTemplateID.String()+"/upgrade", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	type upgradeResp struct {
		Tower ownedTowerResponse `json:"tower"`
	}
	resp := decodeResponse[upgradeResp](t, w)
	if resp.Tower.Current.Level != 2 {
		t.Errorf("level = %d, want 2", resp.Tower.Current.Level)
	}
}

func TestUpgradeTower_MaxLevel_Returns409(t *testing.T) {
	t.Parallel()
	svc := newFakeTowerService()
	svc.levelUpErr = game.ErrMaxLevel
	mux := newTowerMux(svc)
	tok := signedToken(t, uuid.New())

	w := doRequest(mux, http.MethodPost, "/v1/towers/"+shopTemplateID.String()+"/upgrade", tok, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}

	type errResp struct {
		Error struct{ Code string } `json:"error"`
	}
	resp := decodeResponse[errResp](t, w)
	if resp.Error.Code != "max_level" {
		t.Errorf("error code = %q, want max_level", resp.Error.Code)
	}
}

func TestUpgradeTower_NotOwned_Returns409(t *testing.T) {
	t.Parallel()
	svc := newFakeTowerService()
	svc.levelUpErr = game.ErrNotOwned
	mux := newTowerMux(svc)
	tok := signedToken(t, uuid.New())

	w := doRequest(mux, http.MethodPost, "/v1/towers/"+shopTemplateID.String()+"/upgrade", tok, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

func TestUpgradeTower_InsufficientGold_Returns409(t *testing.T) {
	t.Parallel()
	svc := newFakeTowerService()
	svc.levelUpErr = game.ErrInsufficientGold
	mux := newTowerMux(svc)
	tok := signedToken(t, uuid.New())

	w := doRequest(mux, http.MethodPost, "/v1/towers/"+shopTemplateID.String()+"/upgrade", tok, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}
