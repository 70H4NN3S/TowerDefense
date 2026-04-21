package sim

// Step advances the simulation by dt seconds given player input.
// It returns a new State without modifying s; all slice fields are deep-copied
// before any mutation so the caller's State is never aliased.
func Step(s State, in Input, dt float64) State {
	s = deepCopyState(s)
	s = processInputs(s, in)
	s.WaveTime += dt
	s = spawnMonsters(s)
	s = moveMonsters(s, dt)
	s = towerAttacks(s, dt)
	s = advanceWave(s)
	s.Tick++
	return s
}

// initSpawnRecords returns the initial SpawnRecords for a wave, one per group.
func initSpawnRecords(w Wave) []SpawnRecord {
	recs := make([]SpawnRecord, len(w.Groups))
	for i, g := range w.Groups {
		recs[i] = SpawnRecord{
			GroupIdx:   i,
			Spawned:    0,
			NextSpawnT: g.Delay,
		}
	}
	return recs
}

// ── sub-steps ─────────────────────────────────────────────────────────────────

func processInputs(s State, in Input) State {
	for _, pt := range in.PlaceTowers {
		if pt.GoldCost <= 0 || pt.GoldCost > s.Gold {
			continue // refuse negative/zero costs and unaffordable towers
		}
		if pt.Rate <= 0 || pt.Damage < 0 || pt.Range <= 0 {
			continue // invalid tower stats
		}
		if !tileIsValid(s.Map.Tiles, pt.Tile) {
			continue // not a placement tile
		}
		if tileIsOccupied(s.Towers, pt.Tile) {
			continue // already occupied
		}
		s.Gold -= pt.GoldCost
		s.NextID++
		s.Towers = append(s.Towers, Tower{
			ID:         s.NextID,
			TemplateID: pt.TemplateID,
			Tile:       pt.Tile,
			Damage:     pt.Damage,
			Range:      pt.Range,
			Rate:       pt.Rate,
			Cooldown:   0,
		})
	}
	return s
}

func tileIsValid(validTiles []Tile, t Tile) bool {
	for _, vt := range validTiles {
		if vt.Col == t.Col && vt.Row == t.Row {
			return true
		}
	}
	return false
}

func tileIsOccupied(towers []Tower, t Tile) bool {
	for _, tw := range towers {
		if tw.Tile.Col == t.Col && tw.Tile.Row == t.Row {
			return true
		}
	}
	return false
}

// spawnEpsilon is the tolerance applied to WaveTime comparisons. It absorbs
// the floating-point accumulation that arises when WaveTime is built up by
// repeated addition of small dt values. 1 µs is below any game-meaningful
// time granularity.
const spawnEpsilon = 1e-6

func spawnMonsters(s State) State {
	if s.WaveIdx >= len(s.Waves) {
		return s
	}
	wave := s.Waves[s.WaveIdx]
	for i := range s.SpawnRecords {
		rec := &s.SpawnRecords[i]
		group := wave.Groups[rec.GroupIdx]
		for rec.Spawned < group.Count && s.WaveTime+spawnEpsilon >= rec.NextSpawnT {
			s.NextID++
			s.Monsters = append(s.Monsters, Monster{
				ID:       s.NextID,
				MaxHP:    group.MaxHP,
				HP:       group.MaxHP,
				Speed:    group.Speed,
				Reward:   group.Reward,
				Progress: 0,
				Alive:    true,
			})
			rec.Spawned++
			if rec.Spawned < group.Count {
				rec.NextSpawnT += group.Interval
			}
		}
	}
	return s
}

func moveMonsters(s State, dt float64) State {
	pathLen := PathLength(s.Map.Waypoints)
	for i := range s.Monsters {
		m := &s.Monsters[i]
		if !m.Alive {
			continue
		}
		m.Progress += m.Speed * dt
		if m.Progress >= pathLen {
			m.Alive = false
			if s.GateHP > 0 {
				s.GateHP--
			}
		}
	}
	return s
}

func towerAttacks(s State, dt float64) State {
	for i := range s.Towers {
		tw := &s.Towers[i]
		tw.Cooldown -= dt
		if tw.Cooldown < 0 {
			tw.Cooldown = 0
		}
		if tw.Cooldown > 0 {
			continue
		}
		// Find the alive monster with the highest path progress within range.
		towerPos := TileCenter(tw.Tile)
		targetIdx := -1
		bestProgress := -1.0
		for j, m := range s.Monsters {
			if !m.Alive {
				continue
			}
			monsterPos := PosAtProgress(s.Map.Waypoints, m.Progress)
			if towerPos.DistTo(monsterPos) <= tw.Range && m.Progress > bestProgress {
				bestProgress = m.Progress
				targetIdx = j
			}
		}
		if targetIdx < 0 {
			continue
		}
		target := &s.Monsters[targetIdx]
		target.HP -= tw.Damage
		if target.HP <= 0 {
			target.HP = 0
			target.Alive = false
			s.Gold += target.Reward
		}
		tw.Cooldown = 1.0 / tw.Rate
	}
	return s
}

func advanceWave(s State) State {
	if s.WaveIdx >= len(s.Waves) {
		return s
	}

	// All monsters in the current wave must have been spawned.
	for _, rec := range s.SpawnRecords {
		group := s.Waves[s.WaveIdx].Groups[rec.GroupIdx]
		if rec.Spawned < group.Count {
			return s
		}
	}

	// All spawned monsters must be dead.
	for _, m := range s.Monsters {
		if m.Alive {
			return s
		}
	}

	// Wave is complete: advance.
	s.WaveIdx++
	s.WaveTime = 0
	// Clear the dead monster slice; a fresh wave starts clean.
	s.Monsters = nil
	if s.WaveIdx < len(s.Waves) {
		s.SpawnRecords = initSpawnRecords(s.Waves[s.WaveIdx])
	} else {
		s.SpawnRecords = nil
	}
	return s
}

// ── deep copy ─────────────────────────────────────────────────────────────────

// deepCopyState returns a State whose slice fields do not share backing arrays
// with s, so that Step never aliases the caller's data.
func deepCopyState(s State) State {
	out := s
	out.Map.Waypoints = cloneVec2s(s.Map.Waypoints)
	out.Map.Tiles = cloneTiles(s.Map.Tiles)
	out.Towers = cloneTowers(s.Towers)
	out.Monsters = cloneMonsters(s.Monsters)
	out.Waves = cloneWaves(s.Waves)
	out.SpawnRecords = cloneSpawnRecords(s.SpawnRecords)
	return out
}

func cloneVec2s(vs []Vec2) []Vec2 {
	if vs == nil {
		return nil
	}
	out := make([]Vec2, len(vs))
	copy(out, vs)
	return out
}

func cloneTiles(ts []Tile) []Tile {
	if ts == nil {
		return nil
	}
	out := make([]Tile, len(ts))
	copy(out, ts)
	return out
}

func cloneTowers(ts []Tower) []Tower {
	if ts == nil {
		return nil
	}
	out := make([]Tower, len(ts))
	copy(out, ts)
	return out
}

func cloneMonsters(ms []Monster) []Monster {
	if ms == nil {
		return nil
	}
	out := make([]Monster, len(ms))
	copy(out, ms)
	return out
}

func cloneWaves(ws []Wave) []Wave {
	if ws == nil {
		return nil
	}
	out := make([]Wave, len(ws))
	for i, w := range ws {
		out[i] = w
		if w.Groups != nil {
			out[i].Groups = make([]SpawnGroup, len(w.Groups))
			copy(out[i].Groups, w.Groups)
		}
	}
	return out
}

func cloneSpawnRecords(srs []SpawnRecord) []SpawnRecord {
	if srs == nil {
		return nil
	}
	out := make([]SpawnRecord, len(srs))
	copy(out, srs)
	return out
}
