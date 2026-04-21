package game

import (
	"context"
	"errors"
	"testing"

	"github.com/johannesniedens/towerdefense/internal/uuid"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeTowerStore struct {
	templates []TowerTemplate
	levels    map[string]TowerLevel // key: "templateID:level"
	owned     map[string]OwnedTower // key: "userID:templateID"
}

func newFakeTowerStore() *fakeTowerStore {
	return &fakeTowerStore{
		levels: make(map[string]TowerLevel),
		owned:  make(map[string]OwnedTower),
	}
}

func (f *fakeTowerStore) levelKey(templateID uuid.UUID, level int) string {
	return templateID.String() + ":" + string(rune('0'+level))
}

func (f *fakeTowerStore) ownedKey(userID, templateID uuid.UUID) string {
	return userID.String() + ":" + templateID.String()
}

func (f *fakeTowerStore) ListTemplates(_ context.Context) ([]TowerTemplate, error) {
	return append([]TowerTemplate(nil), f.templates...), nil
}

func (f *fakeTowerStore) GetTemplate(_ context.Context, id uuid.UUID) (TowerTemplate, error) {
	for _, t := range f.templates {
		if t.ID == id {
			return t, nil
		}
	}
	return TowerTemplate{}, ErrTemplateNotFound
}

func (f *fakeTowerStore) GetLevel(_ context.Context, templateID uuid.UUID, level int) (TowerLevel, error) {
	tl, ok := f.levels[f.levelKey(templateID, level)]
	if !ok {
		return TowerLevel{}, ErrTemplateNotFound
	}
	return tl, nil
}

func (f *fakeTowerStore) ListOwned(_ context.Context, userID uuid.UUID) ([]OwnedTower, error) {
	var out []OwnedTower
	for _, o := range f.owned {
		if o.UserID == userID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeTowerStore) GetOwned(_ context.Context, userID, templateID uuid.UUID) (OwnedTower, error) {
	o, ok := f.owned[f.ownedKey(userID, templateID)]
	if !ok {
		return OwnedTower{}, ErrNotOwned
	}
	return o, nil
}

func (f *fakeTowerStore) InsertOwned(_ context.Context, userID, templateID uuid.UUID) (OwnedTower, error) {
	key := f.ownedKey(userID, templateID)
	if _, exists := f.owned[key]; exists {
		return OwnedTower{}, ErrAlreadyOwned
	}
	// Find the template.
	var tmpl TowerTemplate
	for _, t := range f.templates {
		if t.ID == templateID {
			tmpl = t
			break
		}
	}
	lvl := f.levels[f.levelKey(templateID, 1)]
	o := OwnedTower{
		UserID:     userID,
		TemplateID: templateID,
		Level:      1,
		Current:    lvl,
		Template:   tmpl,
	}
	f.owned[key] = o
	return o, nil
}

func (f *fakeTowerStore) IncrementLevel(_ context.Context, userID, templateID uuid.UUID) (OwnedTower, error) {
	key := f.ownedKey(userID, templateID)
	o, ok := f.owned[key]
	if !ok {
		return OwnedTower{}, ErrNotOwned
	}
	if o.Level >= 10 {
		return OwnedTower{}, ErrMaxLevel
	}
	o.Level++
	o.Current = f.levels[f.levelKey(templateID, o.Level)]
	f.owned[key] = o
	return o, nil
}

// fakeResources is a minimal ResourceSpender that tracks balance and
// returns sentinel errors when the balance would go negative.
type fakeResources struct {
	gold     int64
	diamonds int64
}

func (r *fakeResources) SpendGold(_ context.Context, _ uuid.UUID, amount int64) (Profile, error) {
	if r.gold < amount {
		return Profile{}, ErrInsufficientGold
	}
	r.gold -= amount
	return Profile{Gold: r.gold}, nil
}

func (r *fakeResources) SpendDiamonds(_ context.Context, _ uuid.UUID, amount int64) (Profile, error) {
	if r.diamonds < amount {
		return Profile{}, ErrInsufficientDiamonds
	}
	r.diamonds -= amount
	return Profile{Diamonds: r.diamonds}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestTemplate(id uuid.UUID, name string, cost int64) TowerTemplate {
	return TowerTemplate{
		ID:           id,
		Name:         name,
		Rarity:       "common",
		BaseDamage:   10,
		BaseRange:    100,
		BaseRate:     5,
		CostDiamonds: cost,
	}
}

func addLevels(store *fakeTowerStore, templateID uuid.UUID) {
	for lvl := 1; lvl <= 10; lvl++ {
		store.levels[store.levelKey(templateID, lvl)] = TowerLevel{
			TemplateID: templateID,
			Level:      lvl,
			GoldCost:   int64(lvl * 100),
			Damage:     int64(10 + lvl*5),
			Range:      100,
			Rate:       5,
		}
	}
}

var (
	userA  = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	tmplID = uuid.MustParse("11111111-1111-4111-8111-111111111111")
)

// ── ListCatalog ───────────────────────────────────────────────────────────────

func TestListCatalog_ReturnsSortedTemplates(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore()
	store.templates = []TowerTemplate{
		newTestTemplate(tmplID, "Archer", 50),
	}
	svc := NewTowerServiceWithStore(store, &fakeResources{})

	templates, err := svc.ListCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("got %d templates, want 1", len(templates))
	}
	if templates[0].Name != "Archer" {
		t.Errorf("name = %q, want Archer", templates[0].Name)
	}
}

func TestListCatalog_EmptyCatalog(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore()
	svc := NewTowerServiceWithStore(store, &fakeResources{})

	templates, err := svc.ListCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("got %d templates, want 0", len(templates))
	}
}

// ── ListCatalogForUser ────────────────────────────────────────────────────────

func TestListCatalogForUser_OwnedFlagCorrect(t *testing.T) {
	t.Parallel()

	tmpl2 := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	store := newFakeTowerStore()
	store.templates = []TowerTemplate{
		newTestTemplate(tmplID, "Archer", 50),
		newTestTemplate(tmpl2, "Cannon", 150),
	}
	addLevels(store, tmplID)
	addLevels(store, tmpl2)

	// pre-own the second template
	store.owned[store.ownedKey(userA, tmpl2)] = OwnedTower{
		UserID: userA, TemplateID: tmpl2, Level: 1,
	}

	svc := NewTowerServiceWithStore(store, &fakeResources{})
	entries, err := svc.ListCatalogForUser(context.Background(), userA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	// entries are ordered by cost_diamonds ascending (fake returns store order)
	if entries[0].Owned {
		t.Errorf("entry[0] (Archer, not owned) Owned = true, want false")
	}
	if !entries[1].Owned {
		t.Errorf("entry[1] (Cannon, owned) Owned = false, want true")
	}
}

// ── Purchase ──────────────────────────────────────────────────────────────────

func TestPurchase_HappyPath(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore()
	store.templates = []TowerTemplate{newTestTemplate(tmplID, "Archer", 50)}
	addLevels(store, tmplID)

	res := &fakeResources{diamonds: 100}
	svc := NewTowerServiceWithStore(store, res)

	tower, err := svc.Purchase(context.Background(), userA, tmplID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tower.Level != 1 {
		t.Errorf("level = %d, want 1", tower.Level)
	}
	if res.diamonds != 50 {
		t.Errorf("diamonds remaining = %d, want 50", res.diamonds)
	}
}

func TestPurchase_InsufficientDiamonds(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore()
	store.templates = []TowerTemplate{newTestTemplate(tmplID, "Archer", 100)}
	addLevels(store, tmplID)

	res := &fakeResources{diamonds: 50}
	svc := NewTowerServiceWithStore(store, res)

	_, err := svc.Purchase(context.Background(), userA, tmplID)
	if !errors.Is(err, ErrInsufficientDiamonds) {
		t.Errorf("err = %v, want ErrInsufficientDiamonds", err)
	}
}

func TestPurchase_AlreadyOwned(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore()
	store.templates = []TowerTemplate{newTestTemplate(tmplID, "Archer", 50)}
	addLevels(store, tmplID)
	// pre-insert ownership
	store.owned[store.ownedKey(userA, tmplID)] = OwnedTower{
		UserID: userA, TemplateID: tmplID, Level: 1,
	}

	res := &fakeResources{diamonds: 200}
	svc := NewTowerServiceWithStore(store, res)

	_, err := svc.Purchase(context.Background(), userA, tmplID)
	if !errors.Is(err, ErrAlreadyOwned) {
		t.Errorf("err = %v, want ErrAlreadyOwned", err)
	}
}

func TestPurchase_UnknownTemplate(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore() // empty catalog
	res := &fakeResources{diamonds: 500}
	svc := NewTowerServiceWithStore(store, res)

	unknown := uuid.MustParse("ffffffff-ffff-4fff-8fff-ffffffffffff")
	_, err := svc.Purchase(context.Background(), userA, unknown)
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("err = %v, want ErrTemplateNotFound", err)
	}
}

// ── LevelUp ───────────────────────────────────────────────────────────────────

func TestLevelUp_HappyPath(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore()
	store.templates = []TowerTemplate{newTestTemplate(tmplID, "Archer", 50)}
	addLevels(store, tmplID)
	store.owned[store.ownedKey(userA, tmplID)] = OwnedTower{
		UserID: userA, TemplateID: tmplID, Level: 1,
		Current:  store.levels[store.levelKey(tmplID, 1)],
		Template: store.templates[0],
	}

	res := &fakeResources{gold: 500}
	svc := NewTowerServiceWithStore(store, res)

	upgraded, err := svc.LevelUp(context.Background(), userA, tmplID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upgraded.Level != 2 {
		t.Errorf("level = %d, want 2", upgraded.Level)
	}
	wantGold := int64(500 - store.levels[store.levelKey(tmplID, 2)].GoldCost)
	if res.gold != wantGold {
		t.Errorf("gold = %d, want %d", res.gold, wantGold)
	}
}

func TestLevelUp_InsufficientGold(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore()
	store.templates = []TowerTemplate{newTestTemplate(tmplID, "Archer", 50)}
	addLevels(store, tmplID)
	store.owned[store.ownedKey(userA, tmplID)] = OwnedTower{
		UserID: userA, TemplateID: tmplID, Level: 1,
	}

	res := &fakeResources{gold: 0}
	svc := NewTowerServiceWithStore(store, res)

	_, err := svc.LevelUp(context.Background(), userA, tmplID)
	if !errors.Is(err, ErrInsufficientGold) {
		t.Errorf("err = %v, want ErrInsufficientGold", err)
	}
}

func TestLevelUp_MaxLevel(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore()
	store.templates = []TowerTemplate{newTestTemplate(tmplID, "Archer", 50)}
	addLevels(store, tmplID)
	store.owned[store.ownedKey(userA, tmplID)] = OwnedTower{
		UserID: userA, TemplateID: tmplID, Level: 10,
	}

	res := &fakeResources{gold: 999999}
	svc := NewTowerServiceWithStore(store, res)

	_, err := svc.LevelUp(context.Background(), userA, tmplID)
	if !errors.Is(err, ErrMaxLevel) {
		t.Errorf("err = %v, want ErrMaxLevel", err)
	}
}

func TestLevelUp_NotOwned(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore()
	store.templates = []TowerTemplate{newTestTemplate(tmplID, "Archer", 50)}
	addLevels(store, tmplID)
	// player owns nothing

	res := &fakeResources{gold: 999999}
	svc := NewTowerServiceWithStore(store, res)

	_, err := svc.LevelUp(context.Background(), userA, tmplID)
	if !errors.Is(err, ErrNotOwned) {
		t.Errorf("err = %v, want ErrNotOwned", err)
	}
}

// ── ListOwned ─────────────────────────────────────────────────────────────────

func TestListOwned_Empty(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore()
	svc := NewTowerServiceWithStore(store, &fakeResources{})

	towers, err := svc.ListOwned(context.Background(), userA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(towers) != 0 {
		t.Errorf("got %d towers, want 0", len(towers))
	}
}

func TestListOwned_ReturnsTowers(t *testing.T) {
	t.Parallel()
	store := newFakeTowerStore()
	store.templates = []TowerTemplate{newTestTemplate(tmplID, "Archer", 50)}
	addLevels(store, tmplID)
	store.owned[store.ownedKey(userA, tmplID)] = OwnedTower{
		UserID: userA, TemplateID: tmplID, Level: 3,
	}

	svc := NewTowerServiceWithStore(store, &fakeResources{})

	towers, err := svc.ListOwned(context.Background(), userA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(towers) != 1 {
		t.Fatalf("got %d towers, want 1", len(towers))
	}
	if towers[0].Level != 3 {
		t.Errorf("level = %d, want 3", towers[0].Level)
	}
}
