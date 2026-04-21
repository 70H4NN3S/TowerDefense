package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johannesniedens/towerdefense/internal/auth"
	"github.com/johannesniedens/towerdefense/internal/game"
	"github.com/johannesniedens/towerdefense/internal/testutil/authtest"
	"github.com/johannesniedens/towerdefense/internal/uuid"
)

// ── fakeProfileService ────────────────────────────────────────────────────────

type fakeProfileService struct {
	profiles map[uuid.UUID]game.Profile
}

func newFakeProfileService() *fakeProfileService {
	return &fakeProfileService{profiles: make(map[uuid.UUID]game.Profile)}
}

func (f *fakeProfileService) seed(p game.Profile) { f.profiles[p.UserID] = p }

func (f *fakeProfileService) CreateProfile(_ context.Context, userID uuid.UUID) (game.Profile, error) {
	p := game.Profile{
		UserID:          userID,
		Energy:          game.EnergyMax,
		EnergyUpdatedAt: time.Now(),
		Level:           1,
	}
	f.profiles[userID] = p
	return p, nil
}

func (f *fakeProfileService) GetProfile(_ context.Context, userID uuid.UUID) (game.Profile, error) {
	p, ok := f.profiles[userID]
	if !ok {
		return game.Profile{}, game.ErrProfileNotFound
	}
	return p, nil
}

func (f *fakeProfileService) UpdateDisplayName(_ context.Context, userID uuid.UUID, name string) (game.Profile, error) {
	p, ok := f.profiles[userID]
	if !ok {
		return game.Profile{}, game.ErrProfileNotFound
	}
	p.DisplayName = name
	f.profiles[userID] = p
	return p, nil
}

func (f *fakeProfileService) UpdateAvatarID(_ context.Context, userID uuid.UUID, avatarID int) (game.Profile, error) {
	p, ok := f.profiles[userID]
	if !ok {
		return game.Profile{}, game.ErrProfileNotFound
	}
	p.AvatarID = avatarID
	f.profiles[userID] = p
	return p, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

var testSecret = []byte("test-secret-key-must-be-long-enough")

// signedToken creates a valid access token for userID, using the auth package
// to mirror production behaviour without a real database.
func signedToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	token, err := auth.SignToken(userID.String(), testSecret, time.Hour)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

// newProfileMux wires a ProfileHandler onto a fresh ServeMux and returns it
// together with the fake service so tests can inspect state.
func newProfileMux(svc *fakeProfileService) *http.ServeMux {
	mux := http.NewServeMux()
	NewProfileHandler(svc, testSecret).Register(mux)
	return mux
}

func doRequest(mux *http.ServeMux, method, path, token, body string) *httptest.ResponseRecorder {
	var reqBody *strings.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		reqBody = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func decodeResponse[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(w.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

// ── GET /v1/me ────────────────────────────────────────────────────────────────

func TestGetMe_NoProfile_CreatesAndReturns201(t *testing.T) {
	t.Parallel()
	svc := newFakeProfileService()
	mux := newProfileMux(svc)

	userID := uuid.New()
	tok := signedToken(t, userID)

	w := doRequest(mux, http.MethodGet, "/v1/me", tok, "")

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	// Profile must now exist in the fake service.
	if _, err := svc.GetProfile(context.Background(), userID); err != nil {
		t.Errorf("profile not created: %v", err)
	}
}

func TestGetMe_ExistingProfile_Returns200(t *testing.T) {
	t.Parallel()
	svc := newFakeProfileService()
	mux := newProfileMux(svc)

	userID := uuid.New()
	svc.seed(game.Profile{
		UserID:          userID,
		DisplayName:     "Hero",
		Gold:            500,
		Diamonds:        20,
		Energy:          game.EnergyMax,
		EnergyUpdatedAt: time.Now(),
		Level:           3,
	})
	tok := signedToken(t, userID)

	w := doRequest(mux, http.MethodGet, "/v1/me", tok, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp profileResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.DisplayName != "Hero" {
		t.Errorf("DisplayName = %q, want %q", resp.DisplayName, "Hero")
	}
	if resp.Gold != 500 {
		t.Errorf("Gold = %d, want 500", resp.Gold)
	}
	if resp.Diamonds != 20 {
		t.Errorf("Diamonds = %d, want 20", resp.Diamonds)
	}
	if resp.Level != 3 {
		t.Errorf("Level = %d, want 3", resp.Level)
	}
	if resp.EnergyMax != game.EnergyMax {
		t.Errorf("EnergyMax = %d, want %d", resp.EnergyMax, game.EnergyMax)
	}
}

func TestGetMe_NoToken_Returns401(t *testing.T) {
	t.Parallel()
	mux := newProfileMux(newFakeProfileService())
	w := doRequest(mux, http.MethodGet, "/v1/me", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestGetMe_InvalidToken_Returns401(t *testing.T) {
	t.Parallel()
	mux := newProfileMux(newFakeProfileService())
	w := doRequest(mux, http.MethodGet, "/v1/me", "not.a.valid.token", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// ── PATCH /v1/me ──────────────────────────────────────────────────────────────

func TestPatchMe_UpdateDisplayName(t *testing.T) {
	t.Parallel()
	svc := newFakeProfileService()
	mux := newProfileMux(svc)

	userID := uuid.New()
	svc.seed(game.Profile{UserID: userID, DisplayName: "Old", EnergyUpdatedAt: time.Now(), Level: 1})
	tok := signedToken(t, userID)

	w := doRequest(mux, http.MethodPatch, "/v1/me", tok, `{"display_name":"New Name"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	resp := decodeResponse[profileResponse](t, w)
	if resp.DisplayName != "New Name" {
		t.Errorf("DisplayName = %q, want %q", resp.DisplayName, "New Name")
	}
}

func TestPatchMe_UpdateAvatarID(t *testing.T) {
	t.Parallel()
	svc := newFakeProfileService()
	mux := newProfileMux(svc)

	userID := uuid.New()
	svc.seed(game.Profile{UserID: userID, AvatarID: 0, EnergyUpdatedAt: time.Now(), Level: 1})
	tok := signedToken(t, userID)

	w := doRequest(mux, http.MethodPatch, "/v1/me", tok, `{"avatar_id":7}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	resp := decodeResponse[profileResponse](t, w)
	if resp.AvatarID != 7 {
		t.Errorf("AvatarID = %d, want 7", resp.AvatarID)
	}
}

func TestPatchMe_EmptyBody_NoChange(t *testing.T) {
	t.Parallel()
	svc := newFakeProfileService()
	mux := newProfileMux(svc)

	userID := uuid.New()
	svc.seed(game.Profile{UserID: userID, DisplayName: "Unchanged", EnergyUpdatedAt: time.Now(), Level: 1})
	tok := signedToken(t, userID)

	w := doRequest(mux, http.MethodPatch, "/v1/me", tok, `{}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	resp := decodeResponse[profileResponse](t, w)
	if resp.DisplayName != "Unchanged" {
		t.Errorf("DisplayName = %q, want %q", resp.DisplayName, "Unchanged")
	}
}

func TestPatchMe_DisplayNameTooLong_Returns400(t *testing.T) {
	t.Parallel()
	svc := newFakeProfileService()
	mux := newProfileMux(svc)

	userID := uuid.New()
	svc.seed(game.Profile{UserID: userID, EnergyUpdatedAt: time.Now(), Level: 1})
	tok := signedToken(t, userID)

	longName := strings.Repeat("a", 33)
	body := `{"display_name":"` + longName + `"}`
	w := doRequest(mux, http.MethodPatch, "/v1/me", tok, body)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestPatchMe_AvatarIDOutOfRange_Returns400(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		avatarID int
	}{
		{"negative", -1},
		{"above max", 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newFakeProfileService()
			mux := newProfileMux(svc)
			userID := uuid.New()
			svc.seed(game.Profile{UserID: userID, EnergyUpdatedAt: time.Now(), Level: 1})
			tok := signedToken(t, userID)

			body := `{"avatar_id":` + intToStr(tt.avatarID) + `}`
			w := doRequest(mux, http.MethodPatch, "/v1/me", tok, body)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
		})
	}
}

func TestPatchMe_ProfileNotFound_Returns404(t *testing.T) {
	t.Parallel()
	svc := newFakeProfileService() // empty — no profile seeded
	mux := newProfileMux(svc)

	// Disable CreateProfile so GET falls through to 404 on PATCH path.
	userID := uuid.New()
	tok := signedToken(t, userID)

	// PATCH requires an existing profile; it does not auto-create.
	w := doRequest(mux, http.MethodPatch, "/v1/me", tok, `{"display_name":"x"}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestPatchMe_NoToken_Returns401(t *testing.T) {
	t.Parallel()
	mux := newProfileMux(newFakeProfileService())
	w := doRequest(mux, http.MethodPatch, "/v1/me", "", `{}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// ── unused import sentinel ────────────────────────────────────────────────────

var _ = authtest.NewService // ensure authtest stays importable

// intToStr converts an int to its decimal string without importing strconv
// (this is test-only code and the number fits in the JSON).
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
