import { api } from './client';
import type {
  AuditEntry,
  AuthTokens,
  ConnectionInfo,
  Device,
  DeviceCommand,
  DeviceState,
  Geofence,
  Position,
  PositionHistory,
  ProtocolDescriptor,
  ProvisioningCommand,
  RawPacket,
  User,
  Vehicle,
  VehicleEvent,
  VehicleView,
} from '@/types';

// ---------------------------------------------------------------------------
// Autenticação
// ---------------------------------------------------------------------------

export const authApi = {
  login: (email: string, password: string) =>
    api.post<AuthTokens>('/api/auth/login', { email, password }),
  logout: (refreshToken: string) => api.post<void>('/api/auth/logout', { refreshToken }),
  me: () => api.get<User>('/api/auth/me'),
  listUsers: () => api.get<User[]>('/api/users'),
  createUser: (input: { email: string; name: string; role: string; password: string }) =>
    api.post<User>('/api/users', input),
};

// ---------------------------------------------------------------------------
// Veículos
// ---------------------------------------------------------------------------

export interface VehicleInput {
  name: string;
  plate?: string;
  brand?: string;
  model?: string;
  year?: number | null;
  color?: string;
  speedLimitKmh?: number | null;
  deviceId?: string | null;
}

export const vehiclesApi = {
  list: () => api.get<VehicleView[]>('/api/vehicles'),
  get: (id: string) => api.get<VehicleView>(`/api/vehicles/${id}`),
  create: (input: VehicleInput) => api.post<Vehicle>('/api/vehicles', input),
  update: (id: string, input: VehicleInput) => api.patch<Vehicle>(`/api/vehicles/${id}`, input),
  remove: (id: string) => api.delete<void>(`/api/vehicles/${id}`),

  position: (id: string) =>
    api.get<{ position: Position | null; state: DeviceState | null; message?: string }>(
      `/api/vehicles/${id}/position`,
    ),

  positions: (id: string, params: { from: string; to: string; limit?: number; simplify?: boolean }) => {
    const query = new URLSearchParams({ from: params.from, to: params.to });
    if (params.limit) query.set('limit', String(params.limit));
    if (params.simplify === false) query.set('simplify', 'false');
    return api.get<PositionHistory>(`/api/vehicles/${id}/positions?${query}`);
  },

  events: (id: string, params: { from?: string; to?: string; limit?: number } = {}) => {
    const query = new URLSearchParams();
    if (params.from) query.set('from', params.from);
    if (params.to) query.set('to', params.to);
    if (params.limit) query.set('limit', String(params.limit));
    return api.get<VehicleEvent[]>(`/api/vehicles/${id}/events?${query}`);
  },

  commands: (id: string, limit = 50) =>
    api.get<DeviceCommand[]>(`/api/vehicles/${id}/commands?limit=${limit}`),
};

// ---------------------------------------------------------------------------
// Comandos
// ---------------------------------------------------------------------------

export const commandsApi = {
  engineCut: (vehicleId: string) =>
    api.post<DeviceCommand>(`/api/vehicles/${vehicleId}/commands/engine-cut`),
  engineResume: (vehicleId: string) =>
    api.post<DeviceCommand>(`/api/vehicles/${vehicleId}/commands/engine-resume`),
  requestPosition: (vehicleId: string) =>
    api.post<DeviceCommand>(`/api/vehicles/${vehicleId}/commands/request-position`),
  requestStatus: (vehicleId: string) =>
    api.post<DeviceCommand>(`/api/vehicles/${vehicleId}/commands/request-status`),
  generic: (vehicleId: string, body: { command: string; params?: Record<string, string>; raw?: string }) =>
    api.post<DeviceCommand>(`/api/vehicles/${vehicleId}/commands`, body),
};

// ---------------------------------------------------------------------------
// Dispositivos
// ---------------------------------------------------------------------------

export interface DeviceInput {
  imei: string;
  model?: string;
  manufacturer?: string;
  protocol?: string;
  firmware?: string;
  phoneNumber?: string;
  apn?: string;
  apnUser?: string;
  apnPassword?: string;
  serverHost?: string;
  serverPort?: number | null;
  reportIntervalSeconds?: number | null;
  heartbeatIntervalSeconds?: number | null;
  commandPassword?: string;
  commandOverrides?: Record<string, string>;
  notes?: string;
}

export const devicesApi = {
  list: () => api.get<Device[]>('/api/devices'),
  get: (id: string) => api.get<Device>(`/api/devices/${id}`),
  create: (input: DeviceInput) => api.post<Device>('/api/devices', input),
  update: (id: string, input: DeviceInput) => api.patch<Device>(`/api/devices/${id}`, input),
  remove: (id: string) => api.delete<void>(`/api/devices/${id}`),
  status: (id: string) =>
    api.get<{
      deviceId: string;
      status: string;
      lastSeenAt: string | null;
      protocol: string;
      connected: boolean;
      state: DeviceState | null;
      connection?: ConnectionInfo;
      lastPosition?: Position;
    }>(`/api/devices/${id}/status`),
  provisioning: (id: string) =>
    api.get<{ device: Device; commands: ProvisioningCommand[]; warning: string }>(
      `/api/devices/${id}/provisioning`,
    ),
};

// ---------------------------------------------------------------------------
// Cercas, eventos e diagnóstico
// ---------------------------------------------------------------------------

export interface GeofenceInput {
  name: string;
  latitude: number;
  longitude: number;
  radiusMeters: number;
  active?: boolean;
}

export const geofencesApi = {
  list: () => api.get<Geofence[]>('/api/geofences'),
  create: (input: GeofenceInput) => api.post<Geofence>('/api/geofences', input),
  update: (id: string, input: GeofenceInput) => api.patch<Geofence>(`/api/geofences/${id}`, input),
  remove: (id: string) => api.delete<void>(`/api/geofences/${id}`),
};

export const eventsApi = {
  recent: (limit = 100) => api.get<VehicleEvent[]>(`/api/events?limit=${limit}`),
};

export const geocodingApi = {
  reverse: (lat: number, lon: number) =>
    api.get<{ address: string }>(`/api/geocoding/reverse?lat=${lat}&lon=${lon}`),
};

export const diagnosticsApi = {
  protocols: () => api.get<ProtocolDescriptor[]>('/api/protocols'),
  connections: () =>
    api.get<{ count: number; connections: ConnectionInfo[] }>('/api/diagnostics/connections'),
  rawPackets: (limit = 100) =>
    api.get<{ packets: RawPacket[]; hint: string }>(`/api/diagnostics/raw-packets?limit=${limit}`),
  auditLogs: (limit = 100) => api.get<AuditEntry[]>(`/api/diagnostics/audit-logs?limit=${limit}`),
};
