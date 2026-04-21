package sim

import (
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// simpleMap returns a minimal Map for testing: a straight horizontal path from
// (0,0.5) to (10,0.5) with gate at (10,0.5), and a single placement tile.
func simpleMap() Map {
	return Map{
		ID:        "test",
		Cols:      11,
		Rows:      2,
		Waypoints: []Vec2{{0, 0.5}, {10, 0.5}},
		Gate:      Vec2{10, 0.5},
		Tiles:     []Tile{{Col: 5, Row: 1}},
	}
}

// oneWave returns a single wave with one group that spawns n monsters.
func oneWave(count int, maxHP int64, speed float64, reward int64) []Wave {
	return []Wave{{
		Number: 1,
		Groups: []SpawnGroup{
			{MaxHP: maxHP, Speed: speed, Reward: reward, Count: count, Interval: 0.5, Delay: 0},
		},
	}}
}

// baseState builds a starting state for testing with the simple map and waves.
func baseState(waves []Wave) State {
	m := simpleMap()
	s := InitialState(m, waves)
	s.Gold = 1000
	s.GateHP = 10
	return s
}

// ── Step: basic properties ────────────────────────────────────────────────────

func TestStep_TickIncrementsBy1(t *testing.T) {
	t.Parallel()
	s := baseState(oneWave(1, 50, 1, 10))
	for i := range 5 {
		before := s.Tick
		s = Step(s, Input{}, 0.1)
		if s.Tick != before+1 {
			t.Fatalf("tick after step %d = %d, want %d", i+1, s.Tick, before+1)
		}
	}
}

func TestStep_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	waves := oneWave(2, 50, 2, 10)
	s := baseState(waves)
	origTick := s.Tick
	_ = Step(s, Input{}, 0.5)
	if s.Tick != origTick {
		t.Error("Step mutated the input State")
	}
	if len(s.Monsters) != 0 {
		t.Error("Step added monsters to the input State")
	}
}

// ── Step: spawning ────────────────────────────────────────────────────────────

func TestStep_SpawnFirstMonster(t *testing.T) {
	t.Parallel()
	// Wave delay=0, so after dt=0.1 the first monster should appear.
	s := baseState(oneWave(3, 50, 1, 10))
	s = Step(s, Input{}, 0.1)
	alive := countAlive(s.Monsters)
	if alive != 1 {
		t.Errorf("alive monsters = %d, want 1 after first tick", alive)
	}
}

func TestStep_SpawnIntervalRespected(t *testing.T) {
	t.Parallel()
	// interval=1.0; use dt=0.5 (exactly representable) to avoid float accumulation.
	// After t=0.5 (step 1, Delay=0): 1 monster. After t=1.0 (step 2): 2 monsters.
	waves := []Wave{{
		Number: 1,
		Groups: []SpawnGroup{
			{MaxHP: 50, Speed: 0, Reward: 0, Count: 3, Interval: 1.0, Delay: 0},
		},
	}}
	s := baseState(waves)
	s = Step(s, Input{}, 0.5) // t=0.5: first monster spawns (Delay=0, NextSpawnT becomes 1.0)
	alive := countAlive(s.Monsters)
	if alive != 1 {
		t.Errorf("alive monsters at t=0.5 = %d, want 1", alive)
	}
	s = Step(s, Input{}, 0.5) // t=1.0: second monster spawns (0.5+0.5=1.0 exactly)
	alive = countAlive(s.Monsters)
	if alive != 2 {
		t.Errorf("alive monsters at t=1.0 = %d, want 2", alive)
	}
}

func TestStep_WaveDelay(t *testing.T) {
	t.Parallel()
	// Use dt=0.5 (exact) to avoid float accumulation. Delay=2.0 seconds.
	waves := []Wave{{
		Number: 1,
		Groups: []SpawnGroup{
			{MaxHP: 50, Speed: 0, Reward: 0, Count: 1, Interval: 1.0, Delay: 2.0},
		},
	}}
	s := baseState(waves)
	s = stepN(s, 3, 0.5) // t=1.5 — before delay
	if countAlive(s.Monsters) != 0 {
		t.Errorf("monster spawned before delay expired")
	}
	s = Step(s, Input{}, 0.5) // t=2.0 — at delay (1.5+0.5=2.0 exactly)
	if countAlive(s.Monsters) != 1 {
		t.Errorf("monster not spawned at delay")
	}
}

// ── Step: movement ────────────────────────────────────────────────────────────

func TestStep_MonsterMovesForward(t *testing.T) {
	t.Parallel()
	waves := oneWave(1, 50, 2.0, 0) // speed = 2 units/s
	s := baseState(waves)
	s = Step(s, Input{}, 0) // spawn at t=0
	if len(s.Monsters) == 0 {
		t.Fatal("no monster spawned")
	}
	prevProgress := s.Monsters[0].Progress
	s = Step(s, Input{}, 0.5)
	if !s.Monsters[0].Alive {
		t.Fatal("monster unexpectedly not alive after 0.5s")
	}
	if s.Monsters[0].Progress <= prevProgress {
		t.Errorf("progress did not increase: was %f, now %f", prevProgress, s.Monsters[0].Progress)
	}
}

func TestStep_MonsterReachesGateReducesGateHP(t *testing.T) {
	t.Parallel()
	// Path is 10 units, speed is 20 → reaches gate in 0.5s.
	waves := oneWave(1, 50, 20, 0)
	s := baseState(waves)
	initialGateHP := s.GateHP
	// Advance far enough for the monster to reach the gate.
	s = stepN(s, 10, 0.1) // 1 second total, monster travels 20 units → past gate
	if s.GateHP >= initialGateHP {
		t.Errorf("GateHP = %d, want < %d after monster reached gate", s.GateHP, initialGateHP)
	}
	// Monster should be marked not-alive.
	for _, m := range s.Monsters {
		if m.Alive {
			t.Error("monster that reached gate is still alive")
		}
	}
}

func TestStep_GateHP_NeverGoesNegative(t *testing.T) {
	t.Parallel()
	// Flood the gate with 20 fast monsters.
	waves := oneWave(20, 50, 100, 0)
	s := baseState(waves)
	s.GateHP = 3
	s = stepN(s, 50, 0.1)
	if s.GateHP < 0 {
		t.Errorf("GateHP = %d, must never be negative", s.GateHP)
	}
}

// ── Step: tower attacks ───────────────────────────────────────────────────────

func TestStep_TowerKillsMonster_GoldAwarded(t *testing.T) {
	t.Parallel()
	// Stationary monster (speed=0), reward=30. Tower has initial cooldown so it
	// won't fire in the same tick as the spawn (dt=0).
	waves := oneWave(1, 50, 0, 30)

	m := simpleMap()
	s := InitialState(m, waves)
	s.Gold = 500
	s.GateHP = 10
	s.NextID++
	s.Towers = []Tower{{
		ID:       s.NextID,
		Tile:     Tile{Col: 5, Row: 1},
		Damage:   100,
		Range:    10,
		Rate:     1,
		Cooldown: 0.5, // won't fire until 0.5s have elapsed
	}}

	// Tick 1 (dt=0): monster spawns; tower cooldown unchanged (0.5s remaining).
	s = Step(s, Input{}, 0)
	if countAlive(s.Monsters) != 1 {
		t.Fatal("monster not spawned")
	}
	goldBefore := s.Gold

	// Advance 0.5s so the tower fires and kills the monster.
	s = Step(s, Input{}, 0.5)
	if countAlive(s.Monsters) != 0 {
		t.Errorf("monster survived tower shot")
	}
	if s.Gold != goldBefore+30 {
		t.Errorf("gold after kill = %d, want %d", s.Gold, goldBefore+30)
	}
}

func TestStep_TowerCooldownRespected(t *testing.T) {
	t.Parallel()
	// Tower rate=0.5 attacks/s → cooldown resets to 2s after each shot.
	// Monster has 200 HP; tower does 100 damage. Should not die in 1 shot.
	waves := oneWave(1, 200, 0, 10)

	m := simpleMap()
	s := InitialState(m, waves)
	s.Gold = 500
	s.GateHP = 10
	s.NextID++
	s.Towers = []Tower{{
		ID:       s.NextID,
		Tile:     Tile{Col: 5, Row: 1},
		Damage:   100,
		Range:    10,
		Rate:     0.5, // 1 shot every 2 seconds
		Cooldown: 0,
	}}

	s = Step(s, Input{}, 0)   // spawn
	s = Step(s, Input{}, 0.1) // tower fires (cooldown resets to 2s, HP→100)
	s = Step(s, Input{}, 0.5) // 0.5s passes, cooldown is ~1.5s
	if countAlive(s.Monsters) == 0 {
		t.Error("monster died before second shot — cooldown not respected")
	}
	if s.Monsters[0].HP != 100 {
		t.Errorf("monster HP = %d after one shot, want 100", s.Monsters[0].HP)
	}
}

func TestStep_MonsterHP_NeverNegative(t *testing.T) {
	t.Parallel()
	// Tower deals huge damage; monster HP must be clamped at 0, never negative.
	waves := oneWave(3, 10, 0, 5)

	m := simpleMap()
	s := InitialState(m, waves)
	s.Gold = 500
	s.GateHP = 10
	s.NextID++
	s.Towers = []Tower{{
		ID: s.NextID, Tile: Tile{Col: 5, Row: 1},
		Damage: 10000, Range: 10, Rate: 5, Cooldown: 0,
	}}

	s = stepN(s, 20, 0.1)
	for _, mo := range s.Monsters {
		if mo.HP < 0 {
			t.Errorf("monster %d has negative HP: %d", mo.ID, mo.HP)
		}
	}
}

// ── Step: tower placement via input ───────────────────────────────────────────

func TestStep_PlaceTower_DeductsGold(t *testing.T) {
	t.Parallel()
	m, waves, _ := LookupMap("alpha")
	// Pick the first valid tile.
	if len(m.Tiles) == 0 {
		t.Fatal("alpha map has no placement tiles")
	}
	s := InitialState(m, waves)
	s.Gold = 500
	s.GateHP = 10

	in := Input{PlaceTowers: []PlaceTower{{
		TemplateID: "t1",
		Tile:       m.Tiles[0],
		Damage:     20,
		Range:      3,
		Rate:       1,
		GoldCost:   100,
	}}}
	s2 := Step(s, in, 0)
	if s2.Gold != 400 {
		t.Errorf("gold after placement = %d, want 400", s2.Gold)
	}
	if len(s2.Towers) != 1 {
		t.Errorf("towers after placement = %d, want 1", len(s2.Towers))
	}
}

func TestStep_PlaceTower_InsufficientGold_Ignored(t *testing.T) {
	t.Parallel()
	m, waves, _ := LookupMap("alpha")
	s := InitialState(m, waves)
	s.Gold = 50
	s.GateHP = 10

	in := Input{PlaceTowers: []PlaceTower{{
		Tile: m.Tiles[0], Damage: 20, Range: 3, Rate: 1, GoldCost: 100,
	}}}
	s2 := Step(s, in, 0)
	if s2.Gold != 50 {
		t.Errorf("gold changed despite insufficient balance: %d", s2.Gold)
	}
	if len(s2.Towers) != 0 {
		t.Errorf("tower placed despite insufficient gold")
	}
}

func TestStep_PlaceTower_OccupiedTile_Ignored(t *testing.T) {
	t.Parallel()
	m, waves, _ := LookupMap("alpha")
	s := InitialState(m, waves)
	s.Gold = 1000
	s.GateHP = 10

	tile := m.Tiles[0]
	s.NextID++
	s.Towers = []Tower{{ID: s.NextID, Tile: tile, Damage: 10, Range: 3, Rate: 1}}

	in := Input{PlaceTowers: []PlaceTower{{
		Tile: tile, Damage: 20, Range: 3, Rate: 1, GoldCost: 100,
	}}}
	s2 := Step(s, in, 0)
	if len(s2.Towers) != 1 {
		t.Errorf("towers = %d, want 1 (duplicate placement should be ignored)", len(s2.Towers))
	}
}

func TestStep_PlaceTower_InvalidTile_Ignored(t *testing.T) {
	t.Parallel()
	waves := oneWave(1, 50, 1, 10)
	s := baseState(waves)

	in := Input{PlaceTowers: []PlaceTower{{
		// Tile (0,0) is on the path in simpleMap — not in the tiles list.
		Tile: Tile{Col: 0, Row: 0}, Damage: 20, Range: 3, Rate: 1, GoldCost: 100,
	}}}
	s2 := Step(s, in, 0)
	if len(s2.Towers) != 0 {
		t.Error("tower placed on invalid (path) tile")
	}
}

// ── Step: wave advancement ────────────────────────────────────────────────────

func TestStep_WaveAdvances_WhenAllMonstersKilled(t *testing.T) {
	t.Parallel()
	// One wave, one monster. Kill it and verify WaveIdx advances to 1.
	waves := oneWave(1, 10, 0, 5)

	m := simpleMap()
	s := InitialState(m, waves)
	s.Gold = 500
	s.GateHP = 10
	s.NextID++
	s.Towers = []Tower{{
		ID: s.NextID, Tile: Tile{Col: 5, Row: 1},
		Damage: 1000, Range: 10, Rate: 10, Cooldown: 0,
	}}

	// Run until wave advances.
	for i := range 50 {
		s = Step(s, Input{}, 0.1)
		if s.WaveIdx == 1 {
			return // pass
		}
		if i == 49 {
			t.Errorf("WaveIdx still 0 after 5s; alive=%d", countAlive(s.Monsters))
		}
	}
}

func TestStep_WaveTime_ResetOnAdvance(t *testing.T) {
	t.Parallel()
	waves := oneWave(1, 10, 0, 5)

	m := simpleMap()
	s := InitialState(m, waves)
	s.Gold = 500
	s.GateHP = 10
	s.NextID++
	s.Towers = []Tower{{
		ID: s.NextID, Tile: Tile{Col: 5, Row: 1},
		Damage: 1000, Range: 10, Rate: 10,
	}}

	for range 50 {
		s = Step(s, Input{}, 0.1)
		if s.WaveIdx == 1 {
			if s.WaveTime >= 0.5 {
				t.Errorf("WaveTime = %f after wave advance; expected near 0", s.WaveTime)
			}
			return
		}
	}
	t.Fatal("wave never advanced")
}

// ── Step: conservation properties ────────────────────────────────────────────

// TestStep_Conservation_GoldNeverDecreasesFromKills verifies that killing
// monsters always credits the correct reward and total gold never drops
// unexpectedly (no tower placement in this scenario).
func TestStep_Conservation_GoldNeverDecreasesFromKills(t *testing.T) {
	t.Parallel()
	waves := oneWave(5, 20, 0, 15) // 5 stationary monsters, 15 gold each

	m := simpleMap()
	s := InitialState(m, waves)
	s.Gold = 0 // start at 0 to make arithmetic easy
	s.GateHP = 10
	s.NextID++
	s.Towers = []Tower{{
		ID: s.NextID, Tile: Tile{Col: 5, Row: 1},
		Damage: 1000, Range: 10, Rate: 10,
	}}

	prev := s.Gold
	for range 100 {
		s = Step(s, Input{}, 0.1)
		if s.Gold < prev {
			t.Errorf("gold decreased from %d to %d at tick %d", prev, s.Gold, s.Tick)
		}
		prev = s.Gold
	}
	// All 5 monsters should have been killed, earning 75 gold.
	if s.Gold != 75 {
		t.Errorf("final gold = %d, want 75", s.Gold)
	}
}

func TestStep_Conservation_DeadMonstersStayDead(t *testing.T) {
	t.Parallel()
	waves := oneWave(3, 30, 0, 5)

	m := simpleMap()
	s := InitialState(m, waves)
	s.Gold = 500
	s.GateHP = 10
	s.NextID++
	s.Towers = []Tower{{
		ID: s.NextID, Tile: Tile{Col: 5, Row: 1},
		Damage: 1000, Range: 10, Rate: 10,
	}}

	dead := make(map[int64]bool)
	for range 60 {
		s = Step(s, Input{}, 0.1)
		for _, mo := range s.Monsters {
			if dead[mo.ID] && mo.Alive {
				t.Errorf("monster %d was dead but is alive again", mo.ID)
			}
			if !mo.Alive {
				dead[mo.ID] = true
			}
		}
	}
}

func TestStep_Conservation_GateHPMonotonic(t *testing.T) {
	t.Parallel()
	// Many fast monsters will reach the gate.
	waves := oneWave(10, 50, 50, 0)
	s := baseState(waves)

	prev := s.GateHP
	for range 30 {
		s = Step(s, Input{}, 0.1)
		if s.GateHP > prev {
			t.Errorf("GateHP increased from %d to %d at tick %d", prev, s.GateHP, s.Tick)
		}
		prev = s.GateHP
	}
}

func TestStep_Conservation_GateHPNeverNegative(t *testing.T) {
	t.Parallel()
	waves := oneWave(100, 50, 100, 0)
	s := baseState(waves)
	s.GateHP = 3

	for range 60 {
		s = Step(s, Input{}, 0.1)
		if s.GateHP < 0 {
			t.Fatalf("GateHP went negative: %d at tick %d", s.GateHP, s.Tick)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func countAlive(monsters []Monster) int {
	n := 0
	for _, m := range monsters {
		if m.Alive {
			n++
		}
	}
	return n
}

func stepN(s State, n int, dt float64) State {
	for range n {
		s = Step(s, Input{}, dt)
	}
	return s
}

// TestLookupMap verifies the registry works end-to-end.
func TestLookupMap_Alpha(t *testing.T) {
	t.Parallel()
	m, waves, ok := LookupMap("alpha")
	if !ok {
		t.Fatal("LookupMap(alpha) returned ok=false")
	}
	if m.ID != "alpha" {
		t.Errorf("map ID = %q, want alpha", m.ID)
	}
	if len(m.Waypoints) < 2 {
		t.Errorf("map has %d waypoints, want >= 2", len(m.Waypoints))
	}
	if len(m.Tiles) == 0 {
		t.Error("alpha map has no placement tiles")
	}
	if len(waves) == 0 {
		t.Error("alpha map has no waves")
	}
}

func TestLookupMap_Unknown(t *testing.T) {
	t.Parallel()
	_, _, ok := LookupMap("does-not-exist")
	if ok {
		t.Error("LookupMap for unknown ID returned ok=true")
	}
}

func TestMaxGoldForMap(t *testing.T) {
	t.Parallel()
	_, waves, _ := LookupMap("alpha")
	got := MaxGoldForMap(waves)
	// Wave rewards: 5×10 + 8×12 + 3×15 + 3×25 + 10×20 + 2×100 + 5×20 = 766
	const want int64 = 766
	if got != want {
		t.Errorf("MaxGoldForMap(alpha) = %d, want %d", got, want)
	}
}
