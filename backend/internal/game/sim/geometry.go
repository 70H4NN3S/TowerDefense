package sim

import "math"

// Add returns v + w.
func (v Vec2) Add(w Vec2) Vec2 { return Vec2{v.X + w.X, v.Y + w.Y} }

// Sub returns v - w.
func (v Vec2) Sub(w Vec2) Vec2 { return Vec2{v.X - w.X, v.Y - w.Y} }

// Scale returns v * s.
func (v Vec2) Scale(s float64) Vec2 { return Vec2{v.X * s, v.Y * s} }

// Len returns the Euclidean length of v.
func (v Vec2) Len() float64 { return math.Sqrt(v.X*v.X + v.Y*v.Y) }

// DistTo returns the Euclidean distance from v to w.
func (v Vec2) DistTo(w Vec2) float64 { return v.Sub(w).Len() }

// TileCenter returns the world-space centre of tile t.
// Each tile occupies a 1×1 cell at (Col, Row), so its centre is at
// (Col+0.5, Row+0.5).
func TileCenter(t Tile) Vec2 {
	return Vec2{float64(t.Col) + 0.5, float64(t.Row) + 0.5}
}

// PathLength returns the total arc-length of the polyline defined by waypoints.
// Returns 0 for fewer than two waypoints.
func PathLength(waypoints []Vec2) float64 {
	total := 0.0
	for i := 1; i < len(waypoints); i++ {
		total += waypoints[i-1].DistTo(waypoints[i])
	}
	return total
}

// PosAtProgress returns the world-space position at distance d along the
// polyline waypoints. d is clamped to [0, PathLength(waypoints)].
func PosAtProgress(waypoints []Vec2, d float64) Vec2 {
	if len(waypoints) == 0 {
		return Vec2{}
	}
	if len(waypoints) == 1 || d <= 0 {
		return waypoints[0]
	}
	remaining := d
	for i := 1; i < len(waypoints); i++ {
		seg := waypoints[i-1].DistTo(waypoints[i])
		if remaining <= seg {
			if seg == 0 {
				return waypoints[i-1]
			}
			t := remaining / seg
			return waypoints[i-1].Add(waypoints[i].Sub(waypoints[i-1]).Scale(t))
		}
		remaining -= seg
	}
	// d ≥ PathLength: clamp to the last waypoint.
	return waypoints[len(waypoints)-1]
}

// distToSegment returns the shortest distance from point p to segment ab.
func distToSegment(a, b, p Vec2) float64 {
	ab := b.Sub(a)
	ap := p.Sub(a)
	lenSq := ab.X*ab.X + ab.Y*ab.Y
	if lenSq == 0 {
		// Degenerate segment (a == b): distance to the point.
		return ap.Len()
	}
	t := (ap.X*ab.X + ap.Y*ab.Y) / lenSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	proj := a.Add(ab.Scale(t))
	return p.Sub(proj).Len()
}

// nearPath reports whether point p is within radius of any segment of the path.
func nearPath(waypoints []Vec2, p Vec2, radius float64) bool {
	for i := 1; i < len(waypoints); i++ {
		if distToSegment(waypoints[i-1], waypoints[i], p) <= radius {
			return true
		}
	}
	return false
}
