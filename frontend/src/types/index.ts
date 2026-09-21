/**
 * Tipos espelhando o JSON da API.
 *
 * Nada aqui conhece TKSTAR, GT06 ou bytes: o frontend só enxerga a telemetria
 * já normalizada pelo backend.
 */

export type DeviceStatus = 'ONLINE' | 'STALE' | 'OFFLINE';

export type CommandStatus =
  | 'PENDING'
  | 'SENDING'
  | 'SENT'
  | 'ACKNOWLEDGED'
  | 'FAILED'
  | 'TIMEOUT'
  | 'REJECTED';

export type CommandType =
  | 'ENGINE_CUT'
  | 'ENGINE_RESUME'
  | 'REQUEST_POSITION'
  | 'REQUEST_STATUS'
  | 'SET_INTERVAL'
  | 'SET_HEARTBEAT'
  | 'SET_SERVER'
  | 'REBOOT'
  | 'CUSTOM';

export type UserRole = 'admin' | 'operator' | 'viewer';

export interface User {
  id: string;
  email: string;
  name: string;
  role: UserRole;
  active: boolean;
  createdAt: string;
}

export interface AuthTokens {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
  user: User;
}

export interface Device {
  id: string;
  imei: string;
  model: string;
  manufacturer: string;
  protocol: string;
  firmware: string;
  phoneNumber: string;
  status: DeviceStatus;
  lastSeenAt: string | null;
  apn: string;
  apnUser: string;
  apnPassword: string;
  serverHost: string;
  serverPort: number | null;
  reportIntervalSeconds: number | null;
  heartbeatIntervalSeconds: number | null;
  commandPassword: string;
  commandOverrides: Record<string, string>;
  notes: string;
  createdAt: string;
  updatedAt: string;
}

export interface Position {
  id: number;
  deviceId: string;
  /** Instante informado pelo aparelho. */
  gpsTimestamp: string;
  /** Instante em que o servidor recebeu — diferente quando há buffer offline. */
  receivedAt: string;
  latitude: number;
  longitude: number;
  speedKmh: number;
  heading: number | null;
  altitude: number | null;
  gpsValid: boolean | null;
  satellites: number | null;
  hdop: number | null;
  acc: boolean | null;
  batteryVoltage: number | null;
  batteryPercent: number | null;
  gsmLevel: number | null;
  relayOn: boolean | null;
  protocol: string;
  source: 'gps' | 'heartbeat' | 'lbs';
  rawPayload?: string;
}

export interface DeviceState {
  deviceId: string;
  overspeed: boolean;
  acc: boolean | null;
  gpsValid: boolean | null;
  relayOn: boolean | null;
  batteryPercent: number | null;
  batteryVoltage: number | null;
  gsmLevel: number | null;
  insideFences: string[];
  lastHeartbeatAt: string | null;
  updatedAt: string;
}

export interface Vehicle {
  id: string;
  name: string;
  plate: string;
  brand: string;
  model: string;
  year: number | null;
  color: string;
  speedLimitKmh: number | null;
  deviceId: string | null;
  createdAt: string;
  updatedAt: string;
}

/** O que a lista do painel consome: veículo com o estado do rastreador junto. */
export interface VehicleView extends Vehicle {
  device: Device | null;
  lastPosition: Position | null;
  state: DeviceState | null;
  connected: boolean;
}

export interface VehicleEvent {
  id: number;
  deviceId: string;
  type: string;
  timestamp: string;
  latitude: number | null;
  longitude: number | null;
  speedKmh: number | null;
  metadata: Record<string, unknown>;
  createdAt: string;
}

export interface DeviceCommand {
  id: string;
  deviceId: string;
  command: CommandType | string;
  payload: string;
  status: CommandStatus;
  correlationKey: number;
  requestedBy: string | null;
  requestedAt: string;
  sentAt: string | null;
  acknowledgedAt: string | null;
  timeoutAt: string | null;
  response: string;
  error: string;
  createdAt: string;
}

export interface Geofence {
  id: string;
  name: string;
  latitude: number;
  longitude: number;
  radiusMeters: number;
  active: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface PositionHistory {
  from: string;
  to: string;
  positions: Position[];
  /** Quantos pontos existem de fato no período. */
  total: number;
  returned: number;
  /** O backend devolveu 1 a cada sampleStep pontos. */
  sampled: boolean;
  sampleStep: number;
  /** Douglas-Peucker aplicado ao traçado. */
  simplified: boolean;
}

export interface ProtocolDescriptor {
  name: string;
  label: string;
  vendor: string;
  /** DOCUMENTED, ASSUMED ou UNKNOWN — ver docs/PROTOCOLS.md. */
  confidence: 'DOCUMENTED' | 'ASSUMED' | 'UNKNOWN';
  commands: CommandType[] | null;
  notes: string;
}

export interface RawPacket {
  id: number;
  remoteAddr: string;
  imei: string;
  protocol: string;
  reason: string;
  payloadHex: string;
  payloadAscii: string;
  byteCount: number;
  receivedAt: string;
}

export interface ConnectionInfo {
  imei: string;
  deviceId: string;
  protocol: string;
  remoteAddr: string;
  connectedAt: string;
  lastSeenAt: string;
}

export interface AuditEntry {
  id: number;
  userId: string | null;
  action: string;
  vehicleId: string | null;
  deviceId: string | null;
  result: string;
  ipAddress: string;
  metadata: Record<string, unknown>;
  createdAt: string;
}

export interface ProvisioningCommand {
  type: CommandType;
  description: string;
  text: string;
  available: boolean;
  reason?: string;
}

/** Eventos recebidos pelo WebSocket (§18). */
export type RealtimeEventType =
  | 'position.updated'
  | 'device.online'
  | 'device.offline'
  | 'device.stale'
  | 'vehicle.event'
  | 'command.sent'
  | 'command.acknowledged'
  | 'command.failed'
  | 'engine.status.changed';

export interface RealtimeMessage<T = unknown> {
  type: RealtimeEventType;
  vehicleId?: string;
  deviceId?: string;
  timestamp: string;
  data?: T;
}
