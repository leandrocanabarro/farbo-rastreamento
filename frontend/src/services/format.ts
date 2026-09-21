/** Formatação de valores exibidos no painel. */

const dateTimeFormatter = new Intl.DateTimeFormat('pt-BR', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
});

const timeFormatter = new Intl.DateTimeFormat('pt-BR', {
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
});

export function formatDateTime(value: string | Date | null | undefined): string {
  if (!value) return '—';
  const date = typeof value === 'string' ? new Date(value) : value;
  if (Number.isNaN(date.getTime())) return '—';
  return dateTimeFormatter.format(date);
}

export function formatTime(value: string | Date | null | undefined): string {
  if (!value) return '—';
  const date = typeof value === 'string' ? new Date(value) : value;
  if (Number.isNaN(date.getTime())) return '—';
  return timeFormatter.format(date);
}

/** Tempo decorrido em texto curto: "agora", "há 3 min", "há 2 h". */
export function formatRelative(value: string | null | undefined): string {
  if (!value) return 'nunca';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';

  const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
  if (seconds < 0) return 'agora';
  if (seconds < 45) return 'agora';
  if (seconds < 90) return 'há 1 min';

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `há ${minutes} min`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `há ${hours} h`;

  const days = Math.floor(hours / 24);
  if (days < 30) return `há ${days} d`;

  return formatDateTime(date);
}

export function formatSpeed(kmh: number | null | undefined): string {
  if (kmh === null || kmh === undefined) return '—';
  return `${kmh.toFixed(0)} km/h`;
}

export function formatCoordinates(lat: number, lon: number): string {
  return `${lat.toFixed(6)}, ${lon.toFixed(6)}`;
}

export function formatDistance(meters: number): string {
  if (meters < 1000) return `${meters.toFixed(0)} m`;
  return `${(meters / 1000).toFixed(1)} km`;
}

export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}min`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h ${minutes % 60}min`;
}

/** Converte o rumo em graus para ponto cardeal. */
export function formatHeading(heading: number | null | undefined): string {
  if (heading === null || heading === undefined) return '—';
  const points = ['N', 'NE', 'L', 'SE', 'S', 'SO', 'O', 'NO'];
  const index = Math.round(heading / 45) % 8;
  return `${points[index]} (${heading.toFixed(0)}°)`;
}

const statusLabels: Record<string, string> = {
  ONLINE: 'Online',
  STALE: 'Sinal fraco',
  OFFLINE: 'Offline',
};

export function formatDeviceStatus(status: string | null | undefined): string {
  if (!status) return 'Sem rastreador';
  return statusLabels[status] ?? status;
}

const commandLabels: Record<string, string> = {
  ENGINE_CUT: 'Desligar motor',
  ENGINE_RESUME: 'Liberar motor',
  REQUEST_POSITION: 'Solicitar posição',
  REQUEST_STATUS: 'Solicitar status',
  SET_INTERVAL: 'Definir intervalo',
  SET_HEARTBEAT: 'Definir heartbeat',
  SET_SERVER: 'Definir servidor',
  REBOOT: 'Reiniciar',
  CUSTOM: 'Comando livre',
};

export function formatCommand(command: string): string {
  return commandLabels[command] ?? command;
}

const commandStatusLabels: Record<string, string> = {
  PENDING: 'Na fila',
  SENDING: 'Enviando',
  SENT: 'Enviado, aguardando confirmação',
  ACKNOWLEDGED: 'Confirmado pelo rastreador',
  FAILED: 'Falhou',
  TIMEOUT: 'Sem resposta',
  REJECTED: 'Recusado pela regra de segurança',
};

export function formatCommandStatus(status: string): string {
  return commandStatusLabels[status] ?? status;
}

const eventLabels: Record<string, string> = {
  IGNITION_ON: 'Ignição ligada',
  IGNITION_OFF: 'Ignição desligada',
  OVERSPEED: 'Excesso de velocidade',
  OVERSPEED_END: 'Velocidade normalizada',
  GEOFENCE_ENTER: 'Entrou na cerca',
  GEOFENCE_EXIT: 'Saiu da cerca',
  VIBRATION: 'Vibração',
  POWER_LOSS: 'Queda de energia',
  LOW_BATTERY: 'Bateria fraca',
  SOS: 'Botão de pânico',
  GPS_LOST: 'GPS perdido',
  GPS_RECOVERED: 'GPS recuperado',
  GSM_LOST: 'Sinal de rede perdido',
  GSM_RECOVERED: 'Sinal de rede recuperado',
  ENGINE_CUT_REQUESTED: 'Corte solicitado',
  ENGINE_CUT_SENT: 'Corte enviado',
  ENGINE_CUT_ACK: 'Corte confirmado',
  ENGINE_RESUME_REQUESTED: 'Liberação solicitada',
  ENGINE_RESUME_SENT: 'Liberação enviada',
  ENGINE_RESUME_ACK: 'Liberação confirmada',
  DEVICE_CONNECTED: 'Rastreador conectou',
  DEVICE_DISCONNECTED: 'Rastreador desconectou',
  DEVICE_STALE: 'Rastreador sem comunicação',
  DEVICE_ALARM: 'Alarme do rastreador',
};

export function formatEvent(type: string): string {
  return eventLabels[type] ?? type;
}

/** Severidade do evento, usada para colorir a lista. */
export function eventSeverity(type: string): 'danger' | 'warning' | 'success' | 'neutral' {
  switch (type) {
    case 'SOS':
    case 'POWER_LOSS':
    case 'ENGINE_CUT_ACK':
      return 'danger';
    case 'OVERSPEED':
    case 'LOW_BATTERY':
    case 'GPS_LOST':
    case 'GSM_LOST':
    case 'DEVICE_DISCONNECTED':
    case 'DEVICE_STALE':
    case 'VIBRATION':
    case 'DEVICE_ALARM':
      return 'warning';
    case 'ENGINE_RESUME_ACK':
    case 'GPS_RECOVERED':
    case 'GSM_RECOVERED':
    case 'DEVICE_CONNECTED':
    case 'OVERSPEED_END':
      return 'success';
    default:
      return 'neutral';
  }
}

/** Data ISO de N horas atrás, para os atalhos de período do histórico. */
export function hoursAgo(hours: number): string {
  return new Date(Date.now() - hours * 3600 * 1000).toISOString();
}

export function nowISO(): string {
  return new Date().toISOString();
}
