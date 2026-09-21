package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/farbo/tracker-platform/backend/internal/audit"
	"github.com/farbo/tracker-platform/backend/internal/auth"
	"github.com/farbo/tracker-platform/backend/internal/database"
	"github.com/farbo/tracker-platform/backend/internal/devices"
	"github.com/farbo/tracker-platform/backend/internal/tracking"
	"github.com/farbo/tracker-platform/backend/internal/vehicles"
)

// vehicleView é o que o painel consome: o veículo com o estado atual do seu
// rastreador já resolvido, para a lista não precisar de N requisições.
type vehicleView struct {
	*vehicles.Vehicle
	Device       *devices.Device    `json:"device"`
	LastPosition *tracking.Position `json:"lastPosition"`
	State        *tracking.State    `json:"state"`
	Connected    bool               `json:"connected"`
}

func (s *Server) handleListVehicles(w http.ResponseWriter, r *http.Request) {
	list, err := s.Vehicles.List(r.Context())
	if err != nil {
		handleStoreError(w, err, "veículos não encontrados")
		return
	}

	allDevices, err := s.Devices.List(r.Context())
	if err != nil {
		handleStoreError(w, err, "dispositivos não encontrados")
		return
	}
	byID := make(map[uuid.UUID]*devices.Device, len(allDevices))
	for _, d := range allDevices {
		byID[d.ID] = d
	}

	positions, err := s.Positions.LatestForAll(r.Context())
	if err != nil {
		handleStoreError(w, err, "posições não encontradas")
		return
	}

	views := make([]vehicleView, 0, len(list))
	for _, vehicle := range list {
		view := vehicleView{Vehicle: vehicle}
		if vehicle.DeviceID != nil {
			device := byID[*vehicle.DeviceID]
			view.Device = device
			view.LastPosition = positions[*vehicle.DeviceID]
			view.State = s.States.Get(*vehicle.DeviceID)
			if device != nil {
				_, view.Connected = s.Conns.Get(device.IMEI)
			}
		}
		views = append(views, view)
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handleGetVehicle(w http.ResponseWriter, r *http.Request) {
	vehicle, _, ok := s.vehicleFromURL(w, r, false)
	if !ok {
		return
	}

	view := vehicleView{Vehicle: vehicle}
	if vehicle.DeviceID != nil {
		device, err := s.Devices.Get(r.Context(), *vehicle.DeviceID)
		if err == nil {
			view.Device = device
			_, view.Connected = s.Conns.Get(device.IMEI)
		}
		if position, err := s.Positions.Latest(r.Context(), *vehicle.DeviceID); err == nil {
			view.LastPosition = position
		}
		view.State = s.States.Get(*vehicle.DeviceID)
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleCreateVehicle(w http.ResponseWriter, r *http.Request) {
	var in vehicles.Input
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	vehicle, err := s.Vehicles.Create(r.Context(), in)
	if err != nil {
		var validation vehicles.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusBadRequest, validation.Message)
			return
		}
		handleStoreError(w, err, "veículo não encontrado")
		return
	}

	s.Ingestor.InvalidateVehicles()
	s.recordAudit(r, audit.ActionVehicleCreated, &vehicle.ID, vehicle.DeviceID,
		map[string]any{"name": vehicle.Name, "plate": vehicle.Plate})
	writeJSON(w, http.StatusCreated, vehicle)
}

func (s *Server) handleUpdateVehicle(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}

	var in vehicles.Input
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	vehicle, err := s.Vehicles.Update(r.Context(), id, in)
	if err != nil {
		var validation vehicles.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusBadRequest, validation.Message)
			return
		}
		handleStoreError(w, err, "veículo não encontrado")
		return
	}

	s.Ingestor.InvalidateVehicles()
	s.recordAudit(r, audit.ActionVehicleUpdated, &vehicle.ID, vehicle.DeviceID,
		map[string]any{"name": vehicle.Name})
	writeJSON(w, http.StatusOK, vehicle)
}

func (s *Server) handleDeleteVehicle(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if err := s.Vehicles.Delete(r.Context(), id); err != nil {
		handleStoreError(w, err, "veículo não encontrado")
		return
	}

	s.Ingestor.InvalidateVehicles()
	s.recordAudit(r, audit.ActionVehicleDeleted, &id, nil, nil)
	writeJSON(w, http.StatusNoContent, nil)
}

// vehicleFromURL resolve o veículo da rota e, quando requireDevice, também o
// rastreador vinculado — que é o que os comandos precisam.
func (s *Server) vehicleFromURL(w http.ResponseWriter, r *http.Request, requireDevice bool) (*vehicles.Vehicle, *devices.Device, bool) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return nil, nil, false
	}

	vehicle, err := s.Vehicles.Get(r.Context(), id)
	if err != nil {
		handleStoreError(w, err, "veículo não encontrado")
		return nil, nil, false
	}

	if !requireDevice {
		return vehicle, nil, true
	}
	if vehicle.DeviceID == nil {
		writeError(w, http.StatusConflict, "este veículo não tem rastreador vinculado")
		return nil, nil, false
	}

	device, err := s.Devices.Get(r.Context(), *vehicle.DeviceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeError(w, http.StatusConflict, "o rastreador vinculado não existe mais")
			return nil, nil, false
		}
		handleStoreError(w, err, "rastreador não encontrado")
		return nil, nil, false
	}
	return vehicle, device, true
}

func (s *Server) recordAudit(r *http.Request, action string, vehicleID, deviceID *uuid.UUID, metadata map[string]any) {
	var userID *uuid.UUID
	if principal, ok := auth.FromContext(r.Context()); ok {
		userID = &principal.UserID
	}
	s.Audit.Record(r.Context(), &audit.Entry{
		UserID: userID, Action: action, VehicleID: vehicleID, DeviceID: deviceID,
		Result: "OK", IPAddress: clientIP(r), Metadata: metadata,
	})
}
