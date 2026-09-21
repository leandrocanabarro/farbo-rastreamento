package tracking

import "math"

const earthRadiusMeters = 6371000.0

// DistanceMeters devolve a distância entre dois pontos (haversine).
func DistanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	dPhi := (lat2 - lat1) * math.Pi / 180
	dLambda := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	return 2 * earthRadiusMeters * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// Simplify aplica Douglas-Peucker ao trajeto, descartando pontos que não mudam
// o desenho da rota além da tolerância informada (em metros).
//
// Serve para o mapa: um trajeto de 5000 pontos numa reta vira meia dúzia sem
// perder a forma. O histórico bruto no banco continua intacto.
func Simplify(positions []*Position, toleranceMeters float64) []*Position {
	if len(positions) < 3 || toleranceMeters <= 0 {
		return positions
	}

	keep := make([]bool, len(positions))
	keep[0] = true
	keep[len(positions)-1] = true
	simplifySegment(positions, 0, len(positions)-1, toleranceMeters, keep)

	out := make([]*Position, 0, len(positions))
	for i, k := range keep {
		if k {
			out = append(out, positions[i])
		}
	}
	return out
}

func simplifySegment(positions []*Position, first, last int, tolerance float64, keep []bool) {
	if last <= first+1 {
		return
	}

	maxDist := 0.0
	maxIdx := first

	for i := first + 1; i < last; i++ {
		d := perpendicularDistance(positions[i], positions[first], positions[last])
		if d > maxDist {
			maxDist = d
			maxIdx = i
		}
	}

	if maxDist <= tolerance {
		return
	}
	keep[maxIdx] = true
	simplifySegment(positions, first, maxIdx, tolerance, keep)
	simplifySegment(positions, maxIdx, last, tolerance, keep)
}

// perpendicularDistance devolve, em metros, a distância do ponto p ao segmento
// a-b. As coordenadas são projetadas localmente em metros (equirretangular),
// aproximação boa o bastante nas distâncias de um trajeto veicular.
func perpendicularDistance(p, a, b *Position) float64 {
	latRef := (a.Latitude + b.Latitude) / 2
	metersPerDegLat := 111132.0
	metersPerDegLon := 111320.0 * math.Cos(latRef*math.Pi/180)

	px := (p.Longitude - a.Longitude) * metersPerDegLon
	py := (p.Latitude - a.Latitude) * metersPerDegLat
	bx := (b.Longitude - a.Longitude) * metersPerDegLon
	by := (b.Latitude - a.Latitude) * metersPerDegLat

	segmentLenSq := bx*bx + by*by
	if segmentLenSq == 0 {
		return math.Hypot(px, py)
	}

	// Projeção escalar do ponto sobre o segmento, limitada às extremidades.
	t := (px*bx + py*by) / segmentLenSq
	t = math.Max(0, math.Min(1, t))

	return math.Hypot(px-t*bx, py-t*by)
}
