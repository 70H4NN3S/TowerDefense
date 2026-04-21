package sim

// allMaps is the registry of built-in maps.
// Each entry is a factory function that returns the Map and its Wave schedule.
var allMaps = map[string]func() (Map, []Wave){
	"alpha": builtinAlpha,
}

// LookupMap returns the Map and Wave schedule for the given ID.
// ok is false if the ID is not in the registry.
func LookupMap(id string) (m Map, waves []Wave, ok bool) {
	fn, found := allMaps[id]
	if !found {
		return Map{}, nil, false
	}
	m, waves = fn()
	return m, waves, true
}

// MaxGoldForMap returns the maximum gold a player can earn by killing all
// monsters across all waves of the map. Used by the replay validator.
func MaxGoldForMap(waves []Wave) int64 {
	var total int64
	for _, w := range waves {
		for _, g := range w.Groups {
			total += g.Reward * int64(g.Count)
		}
	}
	return total
}

// InitialState constructs the starting State for a match on the given map.
// The caller is responsible for setting Gold and GateHP to game-design values.
func InitialState(m Map, waves []Wave) State {
	var recs []SpawnRecord
	if len(waves) > 0 {
		recs = initSpawnRecords(waves[0])
	}
	return State{
		Map:          m,
		Waves:        waves,
		SpawnRecords: recs,
	}
}

// ── Map "alpha" ───────────────────────────────────────────────────────────────

const (
	alphaCols = 14
	alphaRows = 9
)

// alphaWaypoints defines the monster path for map "alpha".
// The path enters from the left at row 2, snakes right→down→left→down, then
// exits at the bottom edge (row 9, which is just past row 8).
var alphaWaypoints = []Vec2{
	{X: 0, Y: 2.5},
	{X: 9.5, Y: 2.5},
	{X: 9.5, Y: 6.5},
	{X: 3.5, Y: 6.5},
	{X: 3.5, Y: 9.0},
}

func builtinAlpha() (Map, []Wave) {
	m := Map{
		ID:        "alpha",
		Cols:      alphaCols,
		Rows:      alphaRows,
		Waypoints: alphaWaypoints,
		Gate:      Vec2{X: 3.5, Y: 9.0},
		Tiles:     computeTiles(alphaCols, alphaRows, alphaWaypoints, 0.75),
	}
	return m, alphaWaves()
}

// computeTiles returns every grid tile whose centre is more than radius
// world-units away from every segment of the path.
func computeTiles(cols, rows int, waypoints []Vec2, radius float64) []Tile {
	tiles := make([]Tile, 0, cols*rows)
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			t := Tile{Col: col, Row: row}
			if !nearPath(waypoints, TileCenter(t), radius) {
				tiles = append(tiles, t)
			}
		}
	}
	return tiles
}

func alphaWaves() []Wave {
	return []Wave{
		// Wave 1 — 5 basic scouts
		{Number: 1, Groups: []SpawnGroup{
			{MaxHP: 50, Speed: 2.0, Reward: 10, Count: 5, Interval: 1.5, Delay: 0},
		}},
		// Wave 2 — 8 slightly faster scouts
		{Number: 2, Groups: []SpawnGroup{
			{MaxHP: 60, Speed: 2.5, Reward: 12, Count: 8, Interval: 1.2, Delay: 0},
		}},
		// Wave 3 — fast scouts + tanky brutes in parallel
		{Number: 3, Groups: []SpawnGroup{
			{MaxHP: 40, Speed: 4.0, Reward: 15, Count: 3, Interval: 1.0, Delay: 0},
			{MaxHP: 150, Speed: 1.5, Reward: 25, Count: 3, Interval: 2.0, Delay: 4.0},
		}},
		// Wave 4 — 10 medium soldiers
		{Number: 4, Groups: []SpawnGroup{
			{MaxHP: 100, Speed: 2.5, Reward: 20, Count: 10, Interval: 1.0, Delay: 0},
		}},
		// Wave 5 — 2 bosses + 5 fast minions
		{Number: 5, Groups: []SpawnGroup{
			{MaxHP: 400, Speed: 1.0, Reward: 100, Count: 2, Interval: 5.0, Delay: 0},
			{MaxHP: 80, Speed: 3.0, Reward: 20, Count: 5, Interval: 0.8, Delay: 2.0},
		}},
	}
}
