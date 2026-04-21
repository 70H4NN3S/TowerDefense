// Package sim implements the deterministic tower-defence simulation.
// It has no I/O, no database access, and no external dependencies.
// All types are plain value types that can be serialised to JSON.
package sim

// Vec2 is a two-dimensional position in world-space.
// One world unit equals one tile width, so the centre of tile (col, row) is
// Vec2{float64(col)+0.5, float64(row)+0.5}.
type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Tile is a grid cell (zero-based column and row) where a tower may be placed.
type Tile struct {
	Col int `json:"col"`
	Row int `json:"row"`
}

// Map describes the static layout of a level: the monster path, the gate, and
// the set of tiles where the player may build towers.
type Map struct {
	ID        string `json:"id"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
	Waypoints []Vec2 `json:"waypoints"` // ordered; monsters travel [0] → [len-1]
	Gate      Vec2   `json:"gate"`      // world-space centre of the gate
	Tiles     []Tile `json:"tiles"`     // valid tower-placement positions
}

// Monster is the live state of one enemy unit.
type Monster struct {
	ID       int64   `json:"id"`
	MaxHP    int64   `json:"max_hp"`
	HP       int64   `json:"hp"`
	Speed    float64 `json:"speed"`    // world-units per second
	Reward   int64   `json:"reward"`   // gold awarded on kill
	Progress float64 `json:"progress"` // total distance travelled along the path
	Alive    bool    `json:"alive"`
}

// Tower is the live state of one defence structure on the map.
type Tower struct {
	ID         int64   `json:"id"`
	TemplateID string  `json:"template_id"`
	Tile       Tile    `json:"tile"`
	Damage     int64   `json:"damage"`
	Range      float64 `json:"range"`    // world-units
	Rate       float64 `json:"rate"`     // attacks per second (must be > 0)
	Cooldown   float64 `json:"cooldown"` // seconds until next attack (0 = ready)
}

// SpawnGroup describes a batch of identical monsters to emit during a wave.
type SpawnGroup struct {
	MaxHP    int64   `json:"max_hp"`
	Speed    float64 `json:"speed"`
	Reward   int64   `json:"reward"`
	Count    int     `json:"count"`
	Interval float64 `json:"interval"` // seconds between spawns within the group
	Delay    float64 `json:"delay"`    // seconds after wave start before first spawn
}

// Wave is the spawn schedule for one enemy wave.
type Wave struct {
	Number int          `json:"number"`
	Groups []SpawnGroup `json:"groups"`
}

// PlaceTower is a player command to build a new tower on a tile.
type PlaceTower struct {
	TemplateID string  `json:"template_id"`
	Tile       Tile    `json:"tile"`
	Damage     int64   `json:"damage"`
	Range      float64 `json:"range"`
	Rate       float64 `json:"rate"`
	GoldCost   int64   `json:"gold_cost"`
}

// Input bundles every player action for one simulation tick.
type Input struct {
	PlaceTowers []PlaceTower `json:"place_towers,omitempty"`
}

// SpawnRecord tracks per-group spawn progress within the current wave.
type SpawnRecord struct {
	GroupIdx   int     `json:"group_idx"`
	Spawned    int     `json:"spawned"`      // monsters spawned from this group so far
	NextSpawnT float64 `json:"next_spawn_t"` // WaveTime at which the next spawn fires
}

// State is the complete, serialisable snapshot of an in-progress match.
// It is a pure value type: Step returns a new State without modifying its input.
type State struct {
	Map          Map           `json:"map"`
	Towers       []Tower       `json:"towers"`
	Monsters     []Monster     `json:"monsters"`
	Waves        []Wave        `json:"waves"`
	WaveIdx      int           `json:"wave_idx"`      // index of the current wave
	WaveTime     float64       `json:"wave_time"`     // elapsed seconds in current wave
	SpawnRecords []SpawnRecord `json:"spawn_records"` // one per group in the current wave
	Gold         int64         `json:"gold"`          // in-match gold balance
	GateHP       int64         `json:"gate_hp"`       // 0 = player lost
	Tick         int64         `json:"tick"`          // increments by 1 each Step call
	NextID       int64         `json:"next_id"`       // monotonically increasing entity counter
}
