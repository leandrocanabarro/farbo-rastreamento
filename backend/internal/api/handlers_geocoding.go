package api

import (
	"net/http"
	"strconv"
)

// handleReverseGeocode traduz coordenadas em um endereço legível.
//
// Não exige nenhum perfil além de estar autenticado: é leitura, cacheada no
// serviço, e não expõe nada sensível.
func (s *Server) handleReverseGeocode(w http.ResponseWriter, r *http.Request) {
	lat, errLat := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, errLon := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	if errLat != nil || errLon != nil {
		writeError(w, http.StatusBadRequest, "parâmetros lat/lon inválidos")
		return
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		writeError(w, http.StatusBadRequest, "coordenadas fora de faixa")
		return
	}

	address, err := s.Geocoder.Reverse(r.Context(), lat, lon)
	if err != nil {
		s.Log.Warn("falha na geocodificação reversa", "err", err)
		writeError(w, http.StatusBadGateway, "não foi possível obter o endereço agora")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"address": address})
}
