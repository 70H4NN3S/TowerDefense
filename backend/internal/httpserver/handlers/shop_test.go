package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/70H4NN3S/TowerDefense/internal/game"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fakeShopService ───────────────────────────────────────────────────────────

type fakeShopService struct {
	catalog []game.CatalogEntry
	owned   map[string]game.OwnedTower // key: "userID:templateID"
	// inject errors
	purchaseErr error
}

func newFakeShopService(templates ...game.TowerTemplate) *fakeShopService {
	entries := make([]game.CatalogEntry, len(templates))
	for i, t := range templates {
		entries[i] = game.CatalogEntry{Template: t}
	}
	return &fakeShopService{
		catalog: entries,
		owned:   make(map[string]game.OwnedTower),
	}
}

func (f *fakeShopService) shopOwnedKey(userID, templateID uuid.UUID) string {
	return userID.String() + ":" + templateID.String()
}

func (f *fakeShopService) ListCatalogForUser(_ context.Context, userID uuid.UUID) ([]game.CatalogEntry, error) {
	out := make([]game.CatalogEntry, len(f.catalog))
	copy(out, f.catalog)
	for i, e := range out {
		_, isOwned := f.owned[f.shopOwnedKey(userID, e.Template.ID)]
		out[i].Owned = isOwned
	}
	return out, nil
}

func (f *fakeShopService) Purchase(_ context.Context, userID, templateID uuid.UUID) (game.OwnedTower, error) {
	if f.purchaseErr != nil {
		return game.OwnedTower{}, f.purchaseErr
	}
	// look up template
	var tmpl game.TowerTemplate
	for _, e := range f.catalog {
		if e.Template.ID == templateID {
			tmpl = e.Template
			break
		}
	}
	tower := game.OwnedTower{UserID: userID, TemplateID: templateID, Level: 1, Template: tmpl}
	f.owned[f.shopOwnedKey(userID, templateID)] = tower
	return tower, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

var (
	shopTemplateID = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	shopTemplate   = game.TowerTemplate{
		ID:           shopTemplateID,
		Name:         "Archer",
		Rarity:       "common",
		BaseDamage:   20,
		BaseRange:    150,
		BaseRate:     10,
		CostDiamonds: 50,
		Description:  "A test tower.",
	}
)

func newShopMux(svc *fakeShopService) *http.ServeMux {
	mux := http.NewServeMux()
	NewShopHandler(svc, testSecret).Register(mux)
	return mux
}

// ── GET /v1/shop/towers ───────────────────────────────────────────────────────

func TestListCatalog_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newShopMux(newFakeShopService(shopTemplate))
	w := doRequest(mux, http.MethodGet, "/v1/shop/towers", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestListCatalog_Returns200WithTowers(t *testing.T) {
	t.Parallel()
	svc := newFakeShopService(shopTemplate)
	mux := newShopMux(svc)

	userID := uuid.New()
	tok := signedToken(t, userID)

	w := doRequest(mux, http.MethodGet, "/v1/shop/towers", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	type shopResp struct {
		Towers []catalogEntryResponse `json:"towers"`
	}
	resp := decodeResponse[shopResp](t, w)
	if len(resp.Towers) != 1 {
		t.Errorf("got %d towers, want 1", len(resp.Towers))
	}
}

func TestListCatalog_OwnedFlagSet(t *testing.T) {
	t.Parallel()
	svc := newFakeShopService(shopTemplate)
	userID := uuid.New()
	// pre-own the tower
	svc.owned[svc.shopOwnedKey(userID, shopTemplateID)] = game.OwnedTower{
		UserID: userID, TemplateID: shopTemplateID, Level: 1,
	}
	mux := newShopMux(svc)
	tok := signedToken(t, userID)

	w := doRequest(mux, http.MethodGet, "/v1/shop/towers", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	type shopResp struct {
		Towers []struct {
			Owned bool `json:"owned"`
		} `json:"towers"`
	}
	resp := decodeResponse[shopResp](t, w)
	if len(resp.Towers) == 0 {
		t.Fatal("expected at least one tower")
	}
	if !resp.Towers[0].Owned {
		t.Errorf("owned = false, want true for pre-owned tower")
	}
}

// ── POST /v1/shop/towers/{id}/buy ─────────────────────────────────────────────

func TestBuyTower_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newShopMux(newFakeShopService(shopTemplate))
	w := doRequest(mux, http.MethodPost, "/v1/shop/towers/"+shopTemplateID.String()+"/buy", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestBuyTower_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()
	mux := newShopMux(newFakeShopService(shopTemplate))
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/shop/towers/not-a-uuid/buy", tok, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestBuyTower_HappyPath_Returns201(t *testing.T) {
	t.Parallel()
	svc := newFakeShopService(shopTemplate)
	mux := newShopMux(svc)
	userID := uuid.New()
	tok := signedToken(t, userID)

	w := doRequest(mux, http.MethodPost, "/v1/shop/towers/"+shopTemplateID.String()+"/buy", tok, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	// Tower should now be in the fake's owned map.
	if _, ok := svc.owned[svc.shopOwnedKey(userID, shopTemplateID)]; !ok {
		t.Error("tower not recorded as owned after purchase")
	}
}

func TestBuyTower_AlreadyOwned_Returns409(t *testing.T) {
	t.Parallel()
	svc := newFakeShopService(shopTemplate)
	svc.purchaseErr = game.ErrAlreadyOwned
	mux := newShopMux(svc)
	tok := signedToken(t, uuid.New())

	w := doRequest(mux, http.MethodPost, "/v1/shop/towers/"+shopTemplateID.String()+"/buy", tok, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}

	type errResp struct {
		Error struct{ Code string } `json:"error"`
	}
	resp := decodeResponse[errResp](t, w)
	if resp.Error.Code != "already_owned" {
		t.Errorf("error code = %q, want already_owned", resp.Error.Code)
	}
}

func TestBuyTower_InsufficientDiamonds_Returns409(t *testing.T) {
	t.Parallel()
	svc := newFakeShopService(shopTemplate)
	svc.purchaseErr = game.ErrInsufficientDiamonds
	mux := newShopMux(svc)
	tok := signedToken(t, uuid.New())

	w := doRequest(mux, http.MethodPost, "/v1/shop/towers/"+shopTemplateID.String()+"/buy", tok, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

func TestBuyTower_UnknownTemplate_Returns404(t *testing.T) {
	t.Parallel()
	svc := newFakeShopService(shopTemplate)
	svc.purchaseErr = game.ErrTemplateNotFound
	mux := newShopMux(svc)
	tok := signedToken(t, uuid.New())

	w := doRequest(mux, http.MethodPost, "/v1/shop/towers/"+shopTemplateID.String()+"/buy", tok, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
