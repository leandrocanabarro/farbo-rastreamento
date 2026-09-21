-- Esquema inicial da plataforma de rastreamento.
-- gen_random_uuid() é nativo do PostgreSQL 13+.

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL,
    name          TEXT        NOT NULL DEFAULT '',
    role          TEXT        NOT NULL DEFAULT 'viewer'
                  CHECK (role IN ('admin', 'operator', 'viewer')),
    password_hash TEXT        NOT NULL,
    active        BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_users_email ON users (lower(email));

-- Refresh tokens são guardados como hash SHA-256; o valor em claro só existe no cliente.
CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    user_agent TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens (user_id);

CREATE TABLE devices (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    imei         VARCHAR(32) NOT NULL UNIQUE,
    model        VARCHAR(100),
    manufacturer VARCHAR(100),
    protocol     VARCHAR(50),
    firmware     VARCHAR(100),
    phone_number VARCHAR(30),

    status       VARCHAR(30) NOT NULL DEFAULT 'OFFLINE'
                 CHECK (status IN ('ONLINE', 'STALE', 'OFFLINE')),

    -- Configuração de provisionamento do rastreador (§30).
    apn             VARCHAR(100),
    apn_user        VARCHAR(100),
    apn_password    VARCHAR(100),
    server_host     VARCHAR(255),
    server_port     INTEGER,
    report_interval_seconds     INTEGER,
    heartbeat_interval_seconds  INTEGER,
    -- Senha exigida por alguns firmwares ao receber comandos.
    command_password VARCHAR(50),
    -- Sobrescreve o texto bruto de um comando: {"ENGINE_CUT": "RELAY,1#"}.
    command_overrides JSONB NOT NULL DEFAULT '{}'::jsonb,

    notes        TEXT,

    last_seen_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_devices_status ON devices (status);

CREATE TABLE vehicles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name  VARCHAR(100) NOT NULL,
    plate VARCHAR(20) UNIQUE,
    brand VARCHAR(100),
    model VARCHAR(100),
    year  INTEGER,
    color VARCHAR(50),

    -- Limite individual de velocidade (§21). NULL usa o padrão global.
    speed_limit_kmh DOUBLE PRECISION,

    device_id UUID UNIQUE REFERENCES devices (id) ON DELETE SET NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_vehicles_device ON vehicles (device_id);

CREATE TABLE positions (
    id BIGSERIAL PRIMARY KEY,

    device_id UUID NOT NULL REFERENCES devices (id) ON DELETE CASCADE,

    -- gps_timestamp vem do dispositivo; received_at é do servidor.
    -- São sempre distintos por causa do buffer offline do rastreador (§35).
    gps_timestamp TIMESTAMPTZ NOT NULL,
    received_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    latitude  DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,

    speed_kmh DOUBLE PRECISION NOT NULL DEFAULT 0,
    heading   DOUBLE PRECISION,

    altitude DOUBLE PRECISION,

    gps_valid  BOOLEAN,
    satellites INTEGER,
    hdop       DOUBLE PRECISION,

    acc BOOLEAN,

    battery_voltage DOUBLE PRECISION,
    battery_percent INTEGER,
    gsm_level       INTEGER,

    -- Estado do relé conforme reportado pelo próprio dispositivo, quando disponível.
    relay_on BOOLEAN,

    protocol VARCHAR(50),
    -- Origem do registro: 'gps', 'heartbeat', 'lbs'. Heartbeat não traz coordenada nova.
    source VARCHAR(20) NOT NULL DEFAULT 'gps',

    raw_payload TEXT
);
CREATE INDEX idx_positions_device_timestamp ON positions (device_id, gps_timestamp DESC);
CREATE INDEX idx_positions_received ON positions (received_at DESC);

CREATE TABLE vehicle_events (
    id BIGSERIAL PRIMARY KEY,

    device_id UUID NOT NULL REFERENCES devices (id) ON DELETE CASCADE,

    type VARCHAR(50) NOT NULL,

    timestamp TIMESTAMPTZ NOT NULL,

    latitude  DOUBLE PRECISION,
    longitude DOUBLE PRECISION,

    speed_kmh DOUBLE PRECISION,

    metadata JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_events_device_timestamp ON vehicle_events (device_id, timestamp DESC);
CREATE INDEX idx_events_type ON vehicle_events (type, timestamp DESC);

CREATE TABLE device_commands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    device_id UUID NOT NULL REFERENCES devices (id) ON DELETE CASCADE,

    command VARCHAR(50) NOT NULL,

    payload TEXT,

    status VARCHAR(30) NOT NULL
           CHECK (status IN ('PENDING', 'SENDING', 'SENT', 'ACKNOWLEDGED',
                             'FAILED', 'TIMEOUT', 'REJECTED')),

    -- Chave de correlação enviada dentro do pacote (server flag no GT06),
    -- usada para casar o ACK com este comando exato (§16).
    correlation_key BIGINT NOT NULL,

    requested_by UUID REFERENCES users (id) ON DELETE SET NULL,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    sent_at         TIMESTAMPTZ,
    acknowledged_at TIMESTAMPTZ,
    timeout_at      TIMESTAMPTZ,

    response TEXT,
    error    TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_commands_device ON device_commands (device_id, created_at DESC);
CREATE INDEX idx_commands_correlation ON device_commands (device_id, correlation_key);
CREATE INDEX idx_commands_open ON device_commands (status)
    WHERE status IN ('PENDING', 'SENDING', 'SENT');

CREATE TABLE geofences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name VARCHAR(100) NOT NULL,

    latitude  DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,

    radius_meters DOUBLE PRECISION NOT NULL CHECK (radius_meters > 0),

    active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,

    user_id UUID REFERENCES users (id) ON DELETE SET NULL,

    action VARCHAR(100) NOT NULL,

    vehicle_id UUID REFERENCES vehicles (id) ON DELETE SET NULL,
    device_id  UUID REFERENCES devices (id) ON DELETE SET NULL,

    result     VARCHAR(50),
    ip_address INET,

    metadata JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_created ON audit_logs (created_at DESC);
CREATE INDEX idx_audit_device ON audit_logs (device_id, created_at DESC);

-- Estado derivado por dispositivo, necessário para não gerar eventos repetidos
-- (excesso de velocidade, dentro/fora de cerca, ignição) após reinício do servidor.
-- Telemetria de estado que NÃO é posição: chega em heartbeat/status e por isso
-- não pode virar uma linha em positions com coordenada inventada (§34/§35).
CREATE TABLE device_states (
    device_id      UUID PRIMARY KEY REFERENCES devices (id) ON DELETE CASCADE,

    overspeed      BOOLEAN NOT NULL DEFAULT FALSE,
    acc            BOOLEAN,
    gps_valid      BOOLEAN,
    relay_on       BOOLEAN,

    battery_percent INTEGER,
    battery_voltage DOUBLE PRECISION,
    gsm_level       INTEGER,

    inside_fences  JSONB   NOT NULL DEFAULT '[]'::jsonb,

    last_heartbeat_at TIMESTAMPTZ,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Captura de pacotes que nenhum protocolo registrado reconheceu (§38).
-- É a base para confirmar qual variante o TK970 real está falando.
CREATE TABLE raw_packets (
    id BIGSERIAL PRIMARY KEY,

    remote_addr TEXT NOT NULL,
    imei        VARCHAR(32),
    -- Protocolo que chegou a ser detectado, quando houve detecção mas o parse falhou.
    protocol    VARCHAR(50),
    reason      TEXT NOT NULL,

    payload_hex   TEXT NOT NULL,
    payload_ascii TEXT NOT NULL,
    byte_count    INTEGER NOT NULL,

    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_raw_packets_received ON raw_packets (received_at DESC);
