package game

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// TowerTemplate is the static definition of a tower type.
type TowerTemplate struct {
	ID           uuid.UUID
	Name         string
	Rarity       string
	BaseDamage   int64
	BaseRange    int64
	BaseRate     int64
	CostDiamonds int64
	Description  string
}

// TowerLevel describes the stats and upgrade cost at a particular level.
type TowerLevel struct {
	TemplateID uuid.UUID
	Level      int
	GoldCost   int64
	Damage     int64
	Range      int64
	Rate       int64
}

// OwnedTower combines a template with the player's current level for it.
type OwnedTower struct {
	UserID     uuid.UUID
	TemplateID uuid.UUID
	Level      int
	// Current holds the stats for the player's current level.
	Current TowerLevel
	// Template is the parent template.
	Template TowerTemplate
}

// TowerStore abstracts data-access for TowerService.
// Declared consumer-side so tests can supply a fake.
type TowerStore interface {
	// ListTemplates returns all templates ordered by cost_diamonds ascending.
	ListTemplates(ctx context.Context) ([]TowerTemplate, error)
	// GetTemplate returns a single template by ID.
	GetTemplate(ctx context.Context, id uuid.UUID) (TowerTemplate, error)
	// GetLevel returns the stats row for (templateID, level).
	GetLevel(ctx context.Context, templateID uuid.UUID, level int) (TowerLevel, error)
	// ListOwned returns all owned towers for a user, joined with template +
	// current-level stats.
	ListOwned(ctx context.Context, userID uuid.UUID) ([]OwnedTower, error)
	// GetOwned returns a single owned tower record.
	GetOwned(ctx context.Context, userID, templateID uuid.UUID) (OwnedTower, error)
	// InsertOwned inserts a new owned_towers row at level 1.
	InsertOwned(ctx context.Context, userID, templateID uuid.UUID) (OwnedTower, error)
	// IncrementLevel increments the level of an existing owned tower by 1.
	IncrementLevel(ctx context.Context, userID, templateID uuid.UUID) (OwnedTower, error)
}

// ResourceSpender is the subset of ResourceService that TowerService needs.
type ResourceSpender interface {
	SpendDiamonds(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error)
	SpendGold(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error)
}

// TowerService implements the business rules for the tower catalog and shop.
type TowerService struct {
	store     TowerStore
	resources ResourceSpender
}

// NewTowerService constructs a TowerService backed by pool.
func NewTowerService(pool *pgxpool.Pool, resources ResourceSpender) *TowerService {
	return &TowerService{
		store:     newTowerStore(pool),
		resources: resources,
	}
}

// NewTowerServiceWithStore constructs a TowerService from explicit dependencies.
// Intended for tests.
func NewTowerServiceWithStore(store TowerStore, resources ResourceSpender) *TowerService {
	return &TowerService{store: store, resources: resources}
}

// ListCatalog returns every tower template. The caller receives all templates
// regardless of ownership; use ListCatalogForUser for an owned flag.
func (s *TowerService) ListCatalog(ctx context.Context) ([]TowerTemplate, error) {
	templates, err := s.store.ListTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("list catalog: %w", err)
	}
	return templates, nil
}

// CatalogEntry is a TowerTemplate annotated with whether the user owns it.
type CatalogEntry struct {
	Template TowerTemplate
	Owned    bool
}

// ListCatalogForUser returns the full catalog with an Owned flag per entry.
func (s *TowerService) ListCatalogForUser(ctx context.Context, userID uuid.UUID) ([]CatalogEntry, error) {
	templates, err := s.store.ListTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("list catalog for user: %w", err)
	}

	owned, err := s.store.ListOwned(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list catalog for user: %w", err)
	}

	ownedSet := make(map[uuid.UUID]struct{}, len(owned))
	for _, o := range owned {
		ownedSet[o.TemplateID] = struct{}{}
	}

	entries := make([]CatalogEntry, len(templates))
	for i, t := range templates {
		_, isOwned := ownedSet[t.ID]
		entries[i] = CatalogEntry{Template: t, Owned: isOwned}
	}
	return entries, nil
}

// ListOwned returns all towers owned by userID with current stats.
func (s *TowerService) ListOwned(ctx context.Context, userID uuid.UUID) ([]OwnedTower, error) {
	towers, err := s.store.ListOwned(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list owned: %w", err)
	}
	return towers, nil
}

// Purchase debits cost_diamonds from the player and creates an owned_towers
// row at level 1. Returns ErrAlreadyOwned if the player already owns it.
func (s *TowerService) Purchase(ctx context.Context, userID, templateID uuid.UUID) (OwnedTower, error) {
	tmpl, err := s.store.GetTemplate(ctx, templateID)
	if err != nil {
		return OwnedTower{}, fmt.Errorf("purchase: %w", err)
	}

	_, err = s.resources.SpendDiamonds(ctx, userID, tmpl.CostDiamonds)
	if err != nil {
		return OwnedTower{}, fmt.Errorf("purchase: %w", err)
	}

	tower, err := s.store.InsertOwned(ctx, userID, templateID)
	if err != nil {
		return OwnedTower{}, fmt.Errorf("purchase: %w", err)
	}
	return tower, nil
}

// LevelUp spends the gold required to advance the tower from its current level
// to the next. Returns ErrMaxLevel if the tower is already level 10, and
// ErrNotOwned if the player does not own the tower.
func (s *TowerService) LevelUp(ctx context.Context, userID, templateID uuid.UUID) (OwnedTower, error) {
	current, err := s.store.GetOwned(ctx, userID, templateID)
	if err != nil {
		return OwnedTower{}, fmt.Errorf("level up: %w", err)
	}

	if current.Level >= 10 {
		return OwnedTower{}, ErrMaxLevel
	}

	nextLevel, err := s.store.GetLevel(ctx, templateID, current.Level+1)
	if err != nil {
		return OwnedTower{}, fmt.Errorf("level up: %w", err)
	}

	_, err = s.resources.SpendGold(ctx, userID, nextLevel.GoldCost)
	if err != nil {
		return OwnedTower{}, fmt.Errorf("level up: %w", err)
	}

	upgraded, err := s.store.IncrementLevel(ctx, userID, templateID)
	if err != nil {
		return OwnedTower{}, fmt.Errorf("level up: %w", err)
	}
	return upgraded, nil
}

// ── towerStore ────────────────────────────────────────────────────────────────

type towerStore struct {
	pool *pgxpool.Pool
}

func newTowerStore(pool *pgxpool.Pool) *towerStore {
	return &towerStore{pool: pool}
}

const templateColumns = `
	id::text, name, rarity, base_damage, base_range, base_rate, cost_diamonds, description`

func scanTemplate(row pgx.Row) (TowerTemplate, error) {
	var t TowerTemplate
	var idStr string
	err := row.Scan(&idStr, &t.Name, &t.Rarity, &t.BaseDamage, &t.BaseRange, &t.BaseRate, &t.CostDiamonds, &t.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return TowerTemplate{}, ErrTemplateNotFound
	}
	if err != nil {
		return TowerTemplate{}, fmt.Errorf("scan template: %w", err)
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return TowerTemplate{}, fmt.Errorf("parse template id %q: %w", idStr, err)
	}
	t.ID = id
	return t, nil
}

func (s *towerStore) ListTemplates(ctx context.Context) ([]TowerTemplate, error) {
	const q = `
		SELECT` + templateColumns + `
		FROM   tower_templates
		ORDER BY cost_diamonds ASC, name ASC`

	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var out []TowerTemplate
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	return out, nil
}

func (s *towerStore) GetTemplate(ctx context.Context, id uuid.UUID) (TowerTemplate, error) {
	const q = `
		SELECT` + templateColumns + `
		FROM   tower_templates
		WHERE  id = $1::uuid`

	t, err := scanTemplate(s.pool.QueryRow(ctx, q, id.String()))
	if err != nil {
		return TowerTemplate{}, fmt.Errorf("get template: %w", err)
	}
	return t, nil
}

func (s *towerStore) GetLevel(ctx context.Context, templateID uuid.UUID, level int) (TowerLevel, error) {
	const q = `
		SELECT template_id::text, level, gold_cost, damage, range_, rate
		FROM   tower_levels
		WHERE  template_id = $1::uuid AND level = $2`

	var tl TowerLevel
	var idStr string
	err := s.pool.QueryRow(ctx, q, templateID.String(), level).Scan(
		&idStr, &tl.Level, &tl.GoldCost, &tl.Damage, &tl.Range, &tl.Rate,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return TowerLevel{}, ErrTemplateNotFound
	}
	if err != nil {
		return TowerLevel{}, fmt.Errorf("get level: %w", err)
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return TowerLevel{}, fmt.Errorf("parse level template_id %q: %w", idStr, err)
	}
	tl.TemplateID = id
	return tl, nil
}

// ownedColumns is the SELECT list used when joining owned_towers with
// tower_templates and tower_levels. Order must match scanOwned exactly.
const ownedColumns = `
	ot.user_id::text, ot.template_id::text, ot.level,
	tl.gold_cost, tl.damage, tl.range_, tl.rate,
	tt.name, tt.rarity, tt.base_damage, tt.base_range, tt.base_rate, tt.cost_diamonds, tt.description`

func scanOwned(row pgx.Row) (OwnedTower, error) {
	var o OwnedTower
	var userIDStr, tmplIDStr string
	err := row.Scan(
		&userIDStr, &tmplIDStr, &o.Level,
		&o.Current.GoldCost, &o.Current.Damage, &o.Current.Range, &o.Current.Rate,
		&o.Template.Name, &o.Template.Rarity,
		&o.Template.BaseDamage, &o.Template.BaseRange, &o.Template.BaseRate,
		&o.Template.CostDiamonds, &o.Template.Description,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OwnedTower{}, ErrNotOwned
	}
	if err != nil {
		return OwnedTower{}, fmt.Errorf("scan owned tower: %w", err)
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return OwnedTower{}, fmt.Errorf("parse owned tower user_id %q: %w", userIDStr, err)
	}
	tmplID, err := uuid.Parse(tmplIDStr)
	if err != nil {
		return OwnedTower{}, fmt.Errorf("parse owned tower template_id %q: %w", tmplIDStr, err)
	}
	o.UserID = userID
	o.TemplateID = tmplID
	o.Template.ID = tmplID
	o.Current.TemplateID = tmplID
	o.Current.Level = o.Level
	return o, nil
}

func (s *towerStore) ListOwned(ctx context.Context, userID uuid.UUID) ([]OwnedTower, error) {
	const q = `
		SELECT` + ownedColumns + `
		FROM   owned_towers ot
		JOIN   tower_templates tt ON tt.id = ot.template_id
		JOIN   tower_levels    tl ON tl.template_id = ot.template_id AND tl.level = ot.level
		WHERE  ot.user_id = $1::uuid
		ORDER BY tt.cost_diamonds ASC, tt.name ASC`

	rows, err := s.pool.Query(ctx, q, userID.String())
	if err != nil {
		return nil, fmt.Errorf("list owned: %w", err)
	}
	defer rows.Close()

	var out []OwnedTower
	for rows.Next() {
		o, err := scanOwned(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list owned: %w", err)
	}
	return out, nil
}

func (s *towerStore) GetOwned(ctx context.Context, userID, templateID uuid.UUID) (OwnedTower, error) {
	const q = `
		SELECT` + ownedColumns + `
		FROM   owned_towers ot
		JOIN   tower_templates tt ON tt.id = ot.template_id
		JOIN   tower_levels    tl ON tl.template_id = ot.template_id AND tl.level = ot.level
		WHERE  ot.user_id = $1::uuid AND ot.template_id = $2::uuid`

	o, err := scanOwned(s.pool.QueryRow(ctx, q, userID.String(), templateID.String()))
	if err != nil {
		return OwnedTower{}, fmt.Errorf("get owned: %w", err)
	}
	return o, nil
}

func (s *towerStore) InsertOwned(ctx context.Context, userID, templateID uuid.UUID) (OwnedTower, error) {
	const q = `
		INSERT INTO owned_towers (user_id, template_id)
		VALUES ($1::uuid, $2::uuid)`

	_, err := s.pool.Exec(ctx, q, userID.String(), templateID.String())
	if err != nil {
		if isUniqueViolation(err, "owned_towers_pkey") {
			return OwnedTower{}, ErrAlreadyOwned
		}
		return OwnedTower{}, fmt.Errorf("insert owned: %w", err)
	}
	return s.GetOwned(ctx, userID, templateID)
}

func (s *towerStore) IncrementLevel(ctx context.Context, userID, templateID uuid.UUID) (OwnedTower, error) {
	const q = `
		UPDATE owned_towers
		SET    level = level + 1
		WHERE  user_id = $1::uuid AND template_id = $2::uuid`

	tag, err := s.pool.Exec(ctx, q, userID.String(), templateID.String())
	if err != nil {
		if isCheckViolation(err, "ck_owned_towers_level") {
			return OwnedTower{}, ErrMaxLevel
		}
		return OwnedTower{}, fmt.Errorf("increment level: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return OwnedTower{}, ErrNotOwned
	}
	return s.GetOwned(ctx, userID, templateID)
}

// isUniqueViolation reports whether err is a PostgreSQL unique_violation (23505)
// for the named constraint.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == constraint
}
