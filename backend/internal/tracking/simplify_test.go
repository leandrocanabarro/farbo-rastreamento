package tracking

import (
	"math"
	"testing"
	"time"
)

func line(points [][2]float64) []*Position {
	out := make([]*Position, len(points))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, p := range points {
		out[i] = &Position{
			ID:           int64(i + 1),
			GPSTimestamp: base.Add(time.Duration(i) * time.Second),
			Latitude:     p[0],
			Longitude:    p[1],
		}
	}
	return out
}

func TestSimplifyKeepsEndpoints(t *testing.T) {
	// Reta com ruído desprezível: só as pontas devem sobrar.
	pts := line([][2]float64{
		{-23.5000, -46.6000},
		{-23.5010, -46.6000},
		{-23.5020, -46.6000},
		{-23.5030, -46.6000},
		{-23.5040, -46.6000},
	})

	got := Simplify(pts, 5)

	if len(got) != 2 {
		t.Fatalf("esperava 2 pontos numa reta, recebi %d", len(got))
	}
	if got[0].ID != pts[0].ID || got[1].ID != pts[len(pts)-1].ID {
		t.Fatal("as extremidades do trajeto precisam ser preservadas")
	}
}

func TestSimplifyKeepsCorners(t *testing.T) {
	// Um "L": o vértice não pode ser descartado.
	pts := line([][2]float64{
		{-23.5000, -46.6000},
		{-23.5010, -46.6000},
		{-23.5020, -46.6000},
		{-23.5020, -46.5990},
		{-23.5020, -46.5980},
	})

	got := Simplify(pts, 5)

	if len(got) != 3 {
		t.Fatalf("esperava 3 pontos (início, vértice, fim), recebi %d", len(got))
	}
	if got[1].ID != 3 {
		t.Fatalf("o vértice deveria ser o ponto 3, veio %d", got[1].ID)
	}
}

func TestSimplifyRespectsTolerance(t *testing.T) {
	// Desvio de ~11 m no meio do trajeto.
	pts := line([][2]float64{
		{-23.5000, -46.6000},
		{-23.5001, -46.5999},
		{-23.5000, -46.5998},
	})

	if got := Simplify(pts, 50); len(got) != 2 {
		t.Fatalf("com tolerância de 50 m o desvio deveria sumir, sobraram %d", len(got))
	}
	if got := Simplify(pts, 2); len(got) != 3 {
		t.Fatalf("com tolerância de 2 m o desvio deveria ficar, sobraram %d", len(got))
	}
}

func TestSimplifyShortInputUnchanged(t *testing.T) {
	pts := line([][2]float64{{-23.5, -46.6}, {-23.6, -46.7}})
	if got := Simplify(pts, 5); len(got) != 2 {
		t.Fatalf("trajeto de 2 pontos não deve ser alterado, veio %d", len(got))
	}
	if got := Simplify(nil, 5); got != nil {
		t.Fatal("entrada vazia deve voltar vazia")
	}
}

func TestDistanceMeters(t *testing.T) {
	// Um grau de latitude ~ 111,2 km.
	d := DistanceMeters(0, 0, 1, 0)
	if math.Abs(d-111195) > 500 {
		t.Fatalf("distância inesperada: %.0f m", d)
	}
	if DistanceMeters(-23.5, -46.6, -23.5, -46.6) != 0 {
		t.Fatal("distância de um ponto a ele mesmo deve ser zero")
	}
}
