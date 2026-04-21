package sim

import (
	"math"
	"testing"
)

// ── Vec2 ──────────────────────────────────────────────────────────────────────

func TestVec2_Add(t *testing.T) {
	t.Parallel()
	got := Vec2{1, 2}.Add(Vec2{3, 4})
	if got.X != 4 || got.Y != 6 {
		t.Errorf("Add = %v, want {4 6}", got)
	}
}

func TestVec2_Sub(t *testing.T) {
	t.Parallel()
	got := Vec2{5, 7}.Sub(Vec2{2, 3})
	if got.X != 3 || got.Y != 4 {
		t.Errorf("Sub = %v, want {3 4}", got)
	}
}

func TestVec2_Scale(t *testing.T) {
	t.Parallel()
	got := Vec2{3, 4}.Scale(2)
	if got.X != 6 || got.Y != 8 {
		t.Errorf("Scale = %v, want {6 8}", got)
	}
}

func TestVec2_Len(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		v    Vec2
		want float64
	}{
		{"zero vector", Vec2{0, 0}, 0},
		{"3-4-5 triangle", Vec2{3, 4}, 5},
		{"unit x", Vec2{1, 0}, 1},
		{"unit y", Vec2{0, 1}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.v.Len()
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("Len() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestVec2_DistTo(t *testing.T) {
	t.Parallel()
	got := Vec2{0, 0}.DistTo(Vec2{3, 4})
	if math.Abs(got-5) > 1e-9 {
		t.Errorf("DistTo = %f, want 5", got)
	}
}

// ── TileCenter ────────────────────────────────────────────────────────────────

func TestTileCenter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		tile Tile
		want Vec2
	}{
		{Tile{0, 0}, Vec2{0.5, 0.5}},
		{Tile{2, 3}, Vec2{2.5, 3.5}},
		{Tile{9, 4}, Vec2{9.5, 4.5}},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := TileCenter(tt.tile)
			if got.X != tt.want.X || got.Y != tt.want.Y {
				t.Errorf("TileCenter(%v) = %v, want %v", tt.tile, got, tt.want)
			}
		})
	}
}

// ── PathLength ────────────────────────────────────────────────────────────────

func TestPathLength(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		waypoints []Vec2
		want      float64
	}{
		{"empty", nil, 0},
		{"single point", []Vec2{{1, 2}}, 0},
		{"horizontal", []Vec2{{0, 0}, {5, 0}}, 5},
		{"L-shape", []Vec2{{0, 0}, {3, 0}, {3, 4}}, 7},
		{"3-4-5 segment", []Vec2{{0, 0}, {3, 4}}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := PathLength(tt.waypoints)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("PathLength = %f, want %f", got, tt.want)
			}
		})
	}
}

// ── PosAtProgress ─────────────────────────────────────────────────────────────

func TestPosAtProgress(t *testing.T) {
	t.Parallel()
	waypoints := []Vec2{{0, 0}, {10, 0}, {10, 5}}

	tests := []struct {
		name  string
		d     float64
		wantX float64
		wantY float64
	}{
		{"start (d=0)", 0, 0, 0},
		{"midway first segment", 5, 5, 0},
		{"end of first segment", 10, 10, 0},
		{"midway second segment", 12.5, 10, 2.5},
		{"end (d=PathLength)", 15, 10, 5},
		{"past end", 100, 10, 5},
		{"negative d", -1, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := PosAtProgress(waypoints, tt.d)
			if math.Abs(got.X-tt.wantX) > 1e-9 || math.Abs(got.Y-tt.wantY) > 1e-9 {
				t.Errorf("PosAtProgress(%v) = %v, want {%v %v}", tt.d, got, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestPosAtProgress_EmptyWaypoints(t *testing.T) {
	t.Parallel()
	got := PosAtProgress(nil, 5)
	if got.X != 0 || got.Y != 0 {
		t.Errorf("PosAtProgress(nil) = %v, want {0 0}", got)
	}
}

func TestPosAtProgress_SingleWaypoint(t *testing.T) {
	t.Parallel()
	got := PosAtProgress([]Vec2{{3, 7}}, 99)
	if got.X != 3 || got.Y != 7 {
		t.Errorf("PosAtProgress single = %v, want {3 7}", got)
	}
}

// ── distToSegment ─────────────────────────────────────────────────────────────

func TestDistToSegment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b Vec2
		p    Vec2
		want float64
	}{
		{
			name: "point above midpoint of horizontal segment",
			a:    Vec2{0, 0}, b: Vec2{10, 0}, p: Vec2{5, 3},
			want: 3,
		},
		{
			name: "point past end of segment",
			a:    Vec2{0, 0}, b: Vec2{5, 0}, p: Vec2{8, 0},
			want: 3,
		},
		{
			name: "point before start of segment",
			a:    Vec2{5, 0}, b: Vec2{10, 0}, p: Vec2{2, 0},
			want: 3,
		},
		{
			name: "degenerate segment (a==b)",
			a:    Vec2{3, 4}, b: Vec2{3, 4}, p: Vec2{0, 0},
			want: 5,
		},
		{
			name: "point on segment",
			a:    Vec2{0, 0}, b: Vec2{10, 0}, p: Vec2{7, 0},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := distToSegment(tt.a, tt.b, tt.p)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("distToSegment = %f, want %f", got, tt.want)
			}
		})
	}
}
