# Plataforma de Rastreamento Veicular

Plataforma própria de rastreamento: recebe os dados dos rastreadores por TCP,
interpreta o protocolo, guarda posições e eventos no PostgreSQL e mostra tudo
em um painel React em tempo real — incluindo envio de comandos ao aparelho,
com corte e liberação de motor.

```
TKSTAR / GT06
     ↓  TCP
Protocol Adapter  ──→  Telemetria normalizada
     ↓
Domínio (tracking, events, commands)
     ↓
PostgreSQL / Redis
     ↓
REST + WebSocket
     ↓
React
```

---

## Sumário

- [Antes de começar: o estado dos protocolos](#antes-de-começar-o-estado-dos-protocolos)
- [Subindo tudo](#subindo-tudo)
- [Primeiro acesso](#primeiro-acesso)
- [Cadastrando o rastreador](#cadastrando-o-rastreador)
- [Apontando o TKSTAR para o servidor](#apontando-o-tkstar-para-o-servidor)
- [Descobrindo a variante do seu aparelho](#descobrindo-a-variante-do-seu-aparelho)
- [Simulador](#simulador)
- [Corte de motor: como funciona a trava](#corte-de-motor-como-funciona-a-trava)
- [API](#api)
- [Observabilidade](#observabilidade)
- [Desenvolvimento](#desenvolvimento)
- [Configuração](#configuração)

---

## Antes de começar: o estado dos protocolos

Não existe "o protocolo TKSTAR". O nome cobre aparelhos de fabricantes
diferentes, com firmwares diferentes. Por isso cada adaptador declara o quanto
está confirmado:

| Adaptador | Confiança | O que faz |
| --- | --- | --- |
| `gt06` | **DOCUMENTED** | Protocolo binário GT06/Concox completo: login, posição, heartbeat, alarmes, comandos com correlação de ACK. Boa parte dos aparelhos 4G com relé fala este protocolo. |
| `h02` | **ASSUMED** | Família H02, nas duas formas: binária (`$`) e texto (`*HQ,...#`). É o que a linha TKSTAR fala — TK905, TK915, TK917, TK920. Layout validado contra uma captura real, não contra documentação do fabricante. |
| `tkstar_v1` / `tkstar_v1_8` | **ASSUMED** | Texto `imei:...;` (família GPS103/TK103). Posição, ACC e alarmes por rótulo. |
| `tkstar_v4` | **UNKNOWN** | Esqueleto de captura. Não produz posição nem aceita comando — existe para registrar o tráfego real e dar lugar ao parser correto. |

> **Nem todo aparelho tem relé.** O TK915, por exemplo, é magnético e a
> bateria: não existe saída de corte nele, e o comando de desligar motor
> simplesmente não faz nada. Corte de motor nessa família é documentado para
> modelos com relé (TK920, TK806) — ou, com suporte `DOCUMENTED`, na linha
> Concox/GT06.

O que está marcado como ASSUMED foi inferido de formatos públicos da família,
**não** confirmado contra o firmware do TK910/TK970. Cada campo nessa condição
carrega um `TODO: VERIFY AGAINST DEVICE PROTOCOL` no código.

Consequência prática: **ligue o aparelho e veja o que ele fala** antes de
confiar em qualquer coisa que não seja o `gt06`. A seção
[Descobrindo a variante](#descobrindo-a-variante-do-seu-aparelho) explica como.
Detalhes em [`docs/PROTOCOLS.md`](docs/PROTOCOLS.md).

---

## Subindo tudo

Requisitos: Docker e Docker Compose.

```bash
cp .env.example .env

# Gere um segredo de verdade para o JWT:
openssl rand -base64 48

# Edite .env: POSTGRES_PASSWORD, JWT_SECRET, ADMIN_EMAIL, ADMIN_PASSWORD
nano .env

docker compose up -d --build
```

Sobe seis serviços:

| Serviço | Porta | Para quê |
| --- | --- | --- |
| `frontend` | 3000 | painel web |
| `backend` | 8080 | API REST + WebSocket |
| `backend` | **5000** | **TCP dos rastreadores** |
| `postgres` | 5432 | banco |
| `redis` | 6379 | replicação de eventos entre instâncias (opcional) |
| `prometheus` | 9090 | métricas |
| `grafana` | 3001 | painéis (admin / `GRAFANA_PASSWORD`) |

As migrations rodam sozinhas na primeira subida.

> **A porta 5000 precisa estar acessível pela internet** — é por ela que o chip
> do rastreador conecta. Se o servidor estiver atrás de NAT, redirecione a
> porta; se houver firewall, libere TCP de entrada.

Confira que está de pé:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

---

## Primeiro acesso

O usuário administrador é criado na primeira subida a partir de `ADMIN_EMAIL` e
`ADMIN_PASSWORD` — e **só** se o banco estiver sem nenhum usuário. Abra
<http://localhost:3000> e entre com essas credenciais.

Perfis disponíveis:

| Perfil | Pode |
| --- | --- |
| `admin` | tudo: cadastro, usuários, diagnóstico, comandos |
| `operator` | ver o painel e enviar comandos |
| `viewer` | apenas visualizar |

---

## Cadastrando o rastreador

A ordem importa: **cadastre o IMEI antes de apontar o aparelho para o
servidor**. Tráfego de IMEI desconhecido é recusado e a sessão é encerrada.

1. Painel → **Rastreadores** → *Novo rastreador*
2. Informe o IMEI (15 dígitos, impresso no aparelho)
3. Deixe o protocolo em branco — o servidor detecta no primeiro pacote e grava
4. **Novo veículo** → dê um nome, uma placa e vincule o rastreador

O campo *senha de comando* só é necessário se o firmware exigir autenticação
nos comandos (alguns pedem `DYD,123456#` em vez de `DYD#`).

> **Aparelhos H02 (linha TKSTAR) costumam reportar um identificador curto**, de
> 10 dígitos, e não o IMEI de 15. Cadastre exatamente o que o aparelho manda.
> Para descobrir qual é, deixe-o conectar uma vez e rode
> `docker compose logs backend | grep "não está cadastrado"` — o identificador
> aparece ali. Se estiver mascarado, ponha `LOG_MASK_IMEI=false` e repita.

---

## Apontando o TKSTAR para o servidor

A configuração é feita **no aparelho**, por SMS, com o chip já ativo. Os
comandos variam por modelo — confira no manual do seu. Os formatos mais comuns
na linha TKSTAR:

```text
# APN da operadora (exemplo Vivo)
apn123456 zap.vivo.com.br

# Servidor e porta
adminip123456 <IP_DO_SEU_SERVIDOR> 5000

# Intervalo de envio em segundos
upload123456 30

# Conferir o que o aparelho entendeu
check123456
```

`123456` é a senha padrão de fábrica da maioria desses aparelhos — troque-a.

Para os modelos Concox/GT06, o comando de servidor costuma ser:

```text
SERVER,1,<HOST_OU_IP>,5000,0#
```

O painel monta esse texto para você: **Rastreadores → Configuração**. Ele
apenas mostra o comando; **nada é enviado automaticamente** ao aparelho.

Mais exemplos e o roteiro completo em [`docs/TKSTAR.md`](docs/TKSTAR.md).

---

## Descobrindo a variante do seu aparelho

Se o aparelho conectar e nada aparecer no painel, é porque nenhum adaptador
reconheceu o tráfego. O servidor guarda esses bytes em vez de descartá-los.

1. Ligue a captura no `.env`:

   ```env
   TKSTAR_V4_CAPTURE=true
   ```

   ```bash
   docker compose up -d backend
   ```

2. Deixe o aparelho conectar.

3. Painel → **Diagnóstico → Pacotes não interpretados**, ou direto no banco:

   ```sql
   SELECT received_at, remote_addr, payload_ascii, payload_hex
   FROM raw_packets ORDER BY received_at DESC LIMIT 20;
   ```

4. Leia o que apareceu:

   | O payload começa com | Então |
   | --- | --- |
   | `78 78` ou `79 79` | é GT06 — já funciona, veja os logs para o erro real |
   | `24` (`$`) | H02 binário → `h02`, já funciona |
   | `*HQ,` ou `*TK,` | H02 texto → `h02`, já funciona |
   | `imei:` ou `##,imei:` | família GPS103 → `tkstar_v1` |
   | outra coisa | implemente o parser em `protocol_v4.go`, que tem o passo a passo comentado |

5. Antes de colocar em produção, escreva o teste com os **bytes reais**
   capturados. Os arquivos em `internal/protocols/*/` mostram o padrão.

---

## Simulador

Testa o fluxo inteiro — inclusive corte e liberação de motor — sem hardware.

```bash
cd backend

go run ./cmd/tksim \
  --imei 869247061234567 \
  --host localhost \
  --port 5000 \
  --interval 10
```

Cadastre esse IMEI no painel antes, senão a conexão é recusada (que é o
comportamento correto).

O simulador aceita comandos pela entrada padrão enquanto roda:

```text
acc on          liga a ignição
acc off         desliga a ignição
speed 60        define a velocidade
move            põe o veículo em deslocamento
stop            para o veículo
sos             dispara alarme de pânico
status          mostra o estado atual
quit            encerra
```

Roteiro para ver a trava de segurança funcionando:

1. `speed 60` → tente **Desligar motor** no painel → **recusado**, com o motivo
2. `stop` → tente de novo → o comando sai, o simulador responde
   `DYD=Success!` e o painel mostra *Motor bloqueado*
3. **Liberar motor** → volta ao normal

---

## Corte de motor: como funciona a trava

O frontend **nunca** aciona o relé. Ele manda uma intenção; quem decide é o
backend, com a última posição conhecida em mãos:

```text
React
  → POST /api/vehicles/:id/commands/engine-cut
  → backend busca a última posição
  → velocidade > ENGINE_CUT_MAX_SPEED_KMH?        → REJECTED
  → posição mais velha que ENGINE_CUT_MAX_POSITION_AGE? → REJECTED
  → não há posição conhecida?                     → REJECTED
  → grava o comando, envia ao aparelho, audita
  → aguarda ACK (COMMAND_ACK_TIMEOUT)
  → WebSocket → React
```

O padrão é conservador: **5 km/h**. Um comando recusado também vira registro no
banco e na auditoria — recusa não é silêncio.

Além disso, o firmware dos aparelhos com relé costuma ter a própria proteção e
só engata o corte quando a velocidade cai. As duas travas somam; nenhuma delas
substitui a outra.

Todo comando — enviado, recusado, confirmado ou expirado — vai para
`audit_logs` com usuário, IP, o texto exato mandado ao aparelho e o resultado.

---

## API

Autenticação por JWT. `POST /api/auth/login` devolve `accessToken` (curto) e
`refreshToken` (longo, rotacionado a cada uso).

```http
POST   /api/auth/login
POST   /api/auth/refresh
POST   /api/auth/logout
GET    /api/auth/me

GET    /api/vehicles                     lista com estado do rastreador junto
POST   /api/vehicles                     (admin)
GET    /api/vehicles/:id
PATCH  /api/vehicles/:id                 (admin)
DELETE /api/vehicles/:id                 (admin)

GET    /api/vehicles/:id/position
GET    /api/vehicles/:id/positions       ?from&to&limit&simplify&raw&after
GET    /api/vehicles/:id/events
GET    /api/vehicles/:id/commands

POST   /api/vehicles/:id/commands/engine-cut       (operator+)
POST   /api/vehicles/:id/commands/engine-resume    (operator+)
POST   /api/vehicles/:id/commands/request-position (operator+)
POST   /api/vehicles/:id/commands/request-status   (operator+)
POST   /api/vehicles/:id/commands                  (operator+; CUSTOM é admin)

GET    /api/devices
POST   /api/devices                      (admin)
GET    /api/devices/:id
GET    /api/devices/:id/status
GET    /api/devices/:id/provisioning     comandos de configuração sugeridos
PATCH  /api/devices/:id                  (admin)
DELETE /api/devices/:id                  (admin)

GET    /api/geofences
POST   /api/geofences                    (admin)
PATCH  /api/geofences/:id                (admin)
DELETE /api/geofences/:id                (admin)

GET    /api/events
GET    /api/protocols

GET    /api/diagnostics/connections      (admin)
GET    /api/diagnostics/raw-packets      (admin)
GET    /api/diagnostics/audit-logs       (admin)

GET    /ws?token=<accessToken>           tempo real
```

### Histórico não devolve tudo

`GET /api/vehicles/:id/positions` nunca despeja milhões de pontos. Acima de
`HISTORY_MAX_POINTS` o banco amostra uniformemente (preservando o primeiro e o
último ponto) e a resposta diz o que aconteceu:

```json
{
  "total": 48213,
  "returned": 5000,
  "sampled": true,
  "sampleStep": 10,
  "simplified": true
}
```

Para exportar o histórico bruto, use a paginação por cursor: `?raw=true` e
depois `?after=<último id>`.

### Eventos do WebSocket

```json
{
  "type": "position.updated",
  "vehicleId": "…",
  "deviceId": "…",
  "timestamp": "2026-09-20T13:45:30Z",
  "data": { "latitude": -23.5, "longitude": -46.6, "speedKmh": 42.5, "acc": true }
}
```

Tipos: `position.updated`, `device.online`, `device.offline`, `device.stale`,
`vehicle.event`, `command.sent`, `command.acknowledged`, `command.failed`,
`engine.status.changed`.

---

## Observabilidade

| Endpoint | O que é |
| --- | --- |
| `/health` | o processo está vivo |
| `/ready` | o banco responde e quantas sessões existem |
| `/metrics` | métricas Prometheus |

Métricas principais: `tracker_connections`, `tracker_online_devices`,
`tracker_packets_received_total`, `tracker_packets_invalid_total`,
`tracker_positions_received_total`, `tracker_commands_sent_total`,
`tracker_commands_failed_total`, `tracker_command_latency_seconds`.

O Grafana já sobe com a fonte de dados e o painel **Rastreamento — visão
geral** provisionados: <http://localhost:3001>.

Os logs são JSON estruturado. O IMEI aparece mascarado
(`869247******567`) — desligue com `LOG_MASK_IMEI=false` se precisar depurar.
Senha, token e credencial nunca são registrados.

Tracing distribuído é opcional: aponte `OTEL_EXPORTER_OTLP_ENDPOINT` para um
coletor OTLP. Vazio instala um tracer no-op e o código instrumentado não muda.

---

## Desenvolvimento

```bash
# Banco e cache apenas
docker compose up -d postgres redis

# Backend
cd backend
export POSTGRES_PASSWORD=... JWT_SECRET=... ADMIN_EMAIL=... ADMIN_PASSWORD=...
go run ./cmd/server

# Frontend (proxy para :8080 já configurado)
cd frontend
npm install
npm run dev
```

Testes:

```bash
cd backend
go test ./...          # parsers, enquadramento TCP, regra de corte
go vet ./...
```

A suíte cobre o que costuma quebrar em produção: pacote partido entre leituras,
vários pacotes numa leitura só, CRC inválido, protocolo desconhecido, IMEI
desconhecido, correlação de ACK e cada ramo da trava do corte de motor.

---

## Configuração

Todas as variáveis estão comentadas em [`.env.example`](.env.example). As que
mais importam:

| Variável | Padrão | O que muda |
| --- | --- | --- |
| `TCP_PORT` | `5000` | porta dos rastreadores |
| `ENGINE_CUT_MAX_SPEED_KMH` | `5` | acima disso o corte é recusado |
| `ENGINE_CUT_MAX_POSITION_AGE` | `10m` | posição mais velha recusa o corte |
| `COMMAND_ACK_TIMEOUT` | `15s` | sem resposta, o comando vira `TIMEOUT` |
| `DEVICE_STALE_AFTER` | `2m` | sem pacotes, vira `STALE` |
| `DEVICE_OFFLINE_AFTER` | `5m` | sem pacotes, vira `OFFLINE` |
| `HISTORY_MAX_POINTS` | `5000` | teto de pontos por consulta |
| `DEFAULT_SPEED_LIMIT_KMH` | `0` | `0` desliga o alerta global |
| `GT06_ACK_GPS` | `false` | responder também aos pacotes de posição |
| `H02_BINARY_FRAME_LENGTH` | `0` | tamanho do quadro binário H02; `0` usa heurística |
| `TKSTAR_V4_CAPTURE` | `false` | capturar tráfego não identificado |
| `LOG_MASK_IMEI` | `true` | mascarar IMEI nos logs |

---

## Documentação

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — como as peças se encaixam
- [`docs/PROTOCOLS.md`](docs/PROTOCOLS.md) — o que está confirmado e como
  implementar uma variante nova
- [`docs/TKSTAR.md`](docs/TKSTAR.md) — configuração do aparelho, passo a passo
