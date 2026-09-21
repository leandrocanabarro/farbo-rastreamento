package protocols

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// DDMMToDecimal converte coordenadas no formato NMEA (DDMM.MMMM / DDDMM.MMMM)
// para graus decimais (§33).
//
//	"5257.4318", "N" -> 52 + 57.4318/60 =  52.957196
//	"04633.1234", "W" -> -(46 + 33.1234/60)
//
// O hemisfério aceita N/S/E/W (ou vazio, tratado como positivo).
func DDMMToDecimal(value, hemisphere string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("coordenada vazia")
	}

	// Os minutos sempre ocupam os dois dígitos antes do ponto decimal;
	// o que vier antes são os graus (2 dígitos na latitude, 3 na longitude).
	dot := strings.IndexByte(value, '.')
	if dot < 0 {
		dot = len(value)
	}
	if dot < 3 {
		return 0, fmt.Errorf("coordenada malformada: %q", value)
	}
	degreeDigits := dot - 2

	degrees, err := strconv.ParseFloat(value[:degreeDigits], 64)
	if err != nil {
		return 0, fmt.Errorf("graus inválidos em %q: %w", value, err)
	}
	minutes, err := strconv.ParseFloat(value[degreeDigits:], 64)
	if err != nil {
		return 0, fmt.Errorf("minutos inválidos em %q: %w", value, err)
	}
	if minutes >= 60 {
		return 0, fmt.Errorf("minutos fora de faixa em %q", value)
	}

	decimal := degrees + minutes/60

	switch strings.ToUpper(strings.TrimSpace(hemisphere)) {
	case "S", "W":
		decimal = -decimal
	case "N", "E", "":
	default:
		return 0, fmt.Errorf("hemisfério inválido: %q", hemisphere)
	}
	return decimal, nil
}

// ValidCoordinates recusa pares fora do globo (e o 0,0 clássico de GPS sem fix).
func ValidCoordinates(lat, lon float64) bool {
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) {
		return false
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return false
	}
	return true
}

// KnotsToKmh converte nós para km/h. Nunca armazenamos velocidade em nós (§9).
func KnotsToKmh(knots float64) float64 { return knots * 1.852 }
