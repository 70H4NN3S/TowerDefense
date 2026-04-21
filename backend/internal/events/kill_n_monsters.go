package events

import (
	"encoding/json"
	"fmt"
)

// KillNMonsters is an event Kind where players accumulate a total kill count
// across matches. Config shape:
//
//	{"tiers":[{"threshold":50,"rewards":{"gold":500}},…]}
//
// Progress shape:
//
//	{"count":127}
type KillNMonsters struct{}

type killNMonstersConfig struct {
	Tiers []Tier `json:"tiers"`
}

type killNMonstersProgress struct {
	Count int64 `json:"count"`
}

// UpdateProgress adds monstersKilled to the running count in existing and
// returns the updated progress JSON.
func (KillNMonsters) UpdateProgress(existing json.RawMessage, monstersKilled, _ int, _ bool) (json.RawMessage, error) {
	var p killNMonstersProgress
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &p); err != nil {
			return nil, fmt.Errorf("kill_n_monsters: unmarshal progress: %w", err)
		}
	}
	p.Count += int64(monstersKilled)
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("kill_n_monsters: marshal progress: %w", err)
	}
	return out, nil
}

// ProgressValue returns the current kill count from progress data.
func (KillNMonsters) ProgressValue(data json.RawMessage) (int64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	var p killNMonstersProgress
	if err := json.Unmarshal(data, &p); err != nil {
		return 0, fmt.Errorf("kill_n_monsters: unmarshal progress value: %w", err)
	}
	return p.Count, nil
}

// Tiers parses the event config and returns the ordered reward tiers.
func (KillNMonsters) Tiers(config json.RawMessage) ([]Tier, error) {
	var c killNMonstersConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return nil, fmt.Errorf("kill_n_monsters: unmarshal config: %w", err)
	}
	return c.Tiers, nil
}
