# Protocolos

Este documento existe por causa de uma regra do projeto: **não inventar campos,
comandos ou formatos de pacote**. Quando algo não está confirmado pela
documentação do firmware, isso fica escrito — no código e aqui.

## Os três níveis

| Nível | Significa | O que se pode fazer com ele |
| --- | --- | --- |
| `DOCUMENTED` | Formato conferido contra especificação pública do protocolo. | Operar, inclusive corte de motor. |
| `ASSUMED` | Gramática inferida de formatos conhecidos da mesma família. Não confirmada contra o firmware do aparelho. | Usar para descobrir o formato real; conferir cada campo antes de confiar. |
| `UNKNOWN` | Nada confirmado. Apenas captura o tráfego. | Descobrir o que o aparelho fala. |

O nível aparece em três lugares: no comentário de cabeçalho de cada adaptador,
em `GET /api/protocols` e na tela **Diagnóstico → Protocolos**. No boot, todo
adaptador que não seja `DOCUMENTED` é registrado em nível `WARN`.

---

## Estado atual

### `gt06` — DOCUMENTED

Protocolo binário GT06 / GT06N / Concox.

```text
78 78 | tamanho | protocolo | conteúdo | serial(2) | CRC-ITU(2) | 0D 0A
79 79 | tamanho(2) | …
```

Confirmado e implementado:

- CRC-ITU = CRC-16/X-25 (init `0xFFFF`, refletido, xorout `0xFFFF`).
  O vetor padrão `"123456789" → 0x906E` está fixado em teste.
- `0x01` login — IMEI em BCD de 8 bytes, resposta ecoando o serial.
- `0x12` / `0x22` / `0x2D` / `0xA0` posição — data/hora, satélites,
  lat/lon em unidades de 1/1800000 de grau, velocidade já em km/h, e o campo de
  curso cujos bits 10, 11 e 12 indicam hemisfério norte, longitude oeste e fix
  válido.
- `0x13` heartbeat — estado do terminal, nível de bateria (0–6), sinal GSM
  (0–4). Exige resposta: sem ACK o aparelho derruba a conexão.
- `0x16` / `0x26` / `0x27` alarme — GPS + LBS + estado + código de alarme.
- `0x15` resposta de comando — traz de volta o *server flag* de 4 bytes.
- `0x80` comando do servidor — tamanho + server flag + texto ASCII + idioma.
- `0x8A` sincronismo de hora.

**Correlação de ACK:** o server flag carrega a chave sorteada por comando e
gravada em `device_commands.correlation_key`. A resposta casa com o comando
exato, mesmo com vários comandos abertos ao mesmo tempo.

Comandos (texto ASCII dentro do `0x80`):

| Comando | Texto | Nível |
| --- | --- | --- |
| Corte de motor | `DYD#` / `DYD,<senha>#` | DOCUMENTED |
| Liberar motor | `HFYD#` / `HFYD,<senha>#` | DOCUMENTED |
| Solicitar posição | `WHERE#` | DOCUMENTED |
| Solicitar status | `STATUS#` | DOCUMENTED |
| Reiniciar | `RESET#` | DOCUMENTED |
| Definir servidor | `SERVER,1,<host>,<porta>,0#` | DOCUMENTED |
| Definir intervalo | `TIMER,<s>#` | **ASSUMED** — há firmware que usa `UPLOAD` |
| Definir heartbeat | `HBT,<min>#` | **ASSUMED** |

### `h02` — ASSUMED

Família H02: é o que a linha TKSTAR fala (TK905, TK915, TK917, TK920). Tem
**duas formas que convivem na mesma conexão** — posições em binário, heartbeat
e respostas de comando em texto — e por isso um único adaptador atende as duas.

#### Forma binária (`$`)

```text
$ 5905101893 | 031437 | 101121 | 37 573250 | 02 | 145 037076 | a | 000 | 344 | FF7FFBFF | ...
  id (5)      hhmmss   ddmmaa    lat GG MM   bat  lon GGG MM  flg  vel   rumo  estado     extra
```

Campos, em ordem: marcador, identificador (5 bytes, ou 8 quando o quadro tem
42 bytes), hora/data em BCD, latitude (GG + MM.MMMM), byte de bateria,
longitude (GGG + MM.MMMM, com um dígito dividindo o byte com o campo
seguinte), nibble de flags, velocidade, rumo e palavra de estado de 4 bytes.

Os flags (nibble baixo): bit 1 = fix válido, bit 2 ligado = hemisfério norte,
bit 3 ligado = longitude leste.

**Validação:** todo nibble que deveria ser dígito BCD é conferido, e data,
coordenada, minutos e rumo passam por faixa. Isso é proposital e é a diferença
mais importante em relação à implementação de referência: como o quadro
binário não tem terminador, um enquadramento errado produziria uma posição
plausível e falsa — o sintoma clássico deste protocolo, com veículos saltando
para o meio do oceano. Aqui, quadro mal enquadrado vira erro e vai para
`raw_packets`.

#### Forma de texto (`*`)

```text
*HQ,869247061234567,V1,134530,A,2332.5000,S,04638.0000,W,000.00,000,170926,FFFFFBFF#
*HQ,869247061234567,HTBT,87#          heartbeat
*HQ,869247061234567,V4,S20,134530#    resposta a um comando
```

#### Palavra de estado

Os mesmos 4 bytes aparecem nas duas formas, e agora são decodificados. A
lógica é **invertida**: o bit LIMPO indica alarme.

| Bit | Limpo significa |
| --- | --- |
| 0 | vibração |
| 1 ou 18 | pânico (SOS) |
| 2 | excesso de velocidade |
| 19 | corte de energia |
| 10 | (ligado = ignição ligada) |

#### Comandos

| Comando | Texto | Observação |
| --- | --- | --- |
| Corte de motor | `*HQ,<id>,S20,<hhmmss>,1,1#` | só funciona em modelo com relé |
| Liberar motor | `*HQ,<id>,S20,<hhmmss>,1,0#` | |
| Definir intervalo | `*HQ,<id>,S71,<hhmmss>,22,<s>#` | |

Posição, status, reboot e servidor **não têm texto confirmado** nesta família e
são recusados com `ErrUnsupportedCommand`.

Não há chave de correlação no protocolo: o ACK casa com o comando aberto mais
antigo, e o registro deixa isso explícito na resposta gravada.

#### O que ainda precisa ser confirmado

1. **A unidade do campo de velocidade.** Tratada aqui como nós, seguindo a
   implementação de referência. A captura disponível tem velocidade zero, então
   não desempata. Um erro aqui é 1,852x — e a velocidade é o que decide se o
   corte de motor é autorizado. Confirme com o aparelho em movimento; o valor
   cru fica em `attributes.speedRaw`.
2. **O tamanho do quadro binário.** Sem terminador e sem campo de tamanho, só
   dá para saber sabendo de antemão. O padrão é assumir um quadro por leitura
   TCP; depois de ver o tamanho real (`attributes.frameBytes`), fixe em
   `H02_BINARY_FRAME_LENGTH`.
3. **O identificador.** Estes aparelhos reportam um id curto (10 dígitos no
   exemplo), não o IMEI de 15. Cadastre o que o aparelho manda.

#### Origem do que está implementado

O layout vem de duas fontes independentes que concordam: a implementação de
referência aberta do protocolo e uma captura real de um TK905 4G publicada por
quem tinha o aparelho. A captura decodifica para Melbourne, parado, ignição
desligada — coerente em todos os campos ao mesmo tempo, o que é a evidência de
que a ordem está certa. **Nenhuma das duas é documentação do fabricante**, e
por isso o nível é ASSUMED e não DOCUMENTED.

### `tkstar_v1` / `tkstar_v1_8` — ASSUMED

Texto terminado em `;`, família GPS103/TK103:

```text
##,imei:869247061234567,A;                              → login, resposta "LOAD"
869247061234567;                                        → heartbeat, resposta "ON"
imei:869247061234567,tracker,2509171345,,F,134530.000,A,2332.5000,S,04638.0000,W,0.00,0;
```

O campo de tipo traz o alarme: `tracker`, `acc on`, `acc off`, `help me`,
`low battery`, `move`, `stockade`, `speed`. Rótulo desconhecido não é
adivinhado — vai para `attributes.unmappedType`.

Linha que não bate com o formato volta como `KindOther` com o texto original, e
o pacote é capturado para análise.

Comandos padrão desta variante: `RELAY,1#`, `RELAY,0#`, `WHERE#`, `STATUS#`.
Intervalo, heartbeat, servidor e reboot **não têm texto confirmado** e são
recusados com `ErrUnsupportedCommand` em vez de chutados.

### `tkstar_v4` — UNKNOWN

Esqueleto vazio, ligado por `TKSTAR_V4_CAPTURE=true`. Nunca produz posição e
recusa todo comando. `protocol_v4.go` tem o passo a passo comentado para
preenchê-lo a partir de uma captura real.

---

## Como o tráfego é atendido

```text
bytes do socket
  → buffer acumulado da sessão          (1 Read() ≠ 1 pacote)
  → Detect(): primeiro adaptador que reconhecer o início
  → NextFrame(): onde o quadro começa e termina
  → Parse(): um quadro → TrackerMessage normalizada
  → ACK de volta, se o protocolo exigir
  → Ingestor → estado, histórico, eventos, WebSocket
```

Nada que não seja reconhecido é descartado em silêncio: vai para `raw_packets`
com hexdump e ASCII, e aparece em **Diagnóstico → Pacotes não interpretados**.

O protocolo é decidido **por conexão**, na primeira leitura, e gravado no
cadastro do dispositivo — o que o aparelho realmente fala vale mais do que o
que foi digitado no formulário.

---

## Implementando uma variante nova

`TrackerProtocol` tem cinco métodos:

```go
type TrackerProtocol interface {
    Framer                                      // NextFrame: onde o quadro termina
    Name() string
    Detect(data []byte) bool                    // isto é meu?
    Parse(data []byte) ([]TrackerMessage, error)
    EncodeCommand(command Command) ([]byte, error)
    Describe() Descriptor                       // nível de confiança e comandos
}
```

Roteiro:

1. **Capture o tráfego real.** Sem bytes de verdade, não comece.
2. **Crie o arquivo** em `internal/protocols/<vendor>/`. Copie a estrutura de
   `tkstar/protocol_v3.go` (texto) ou `gt06/` (binário).
3. **Declare o nível** em `Describe()`. Comece em `ASSUMED`.
4. **`Detect` precisa ser específico.** Reconhecer demais faz o adaptador
   roubar tráfego de outro. O `tkstar_v3` exige `*` *e* um IMEI válido no
   segundo campo justamente por isso.
5. **`NextFrame` nunca pode travar.** Devolva `ErrIncompleteFrame` quando
   faltar dado e consuma bytes quando o quadro for inválido, senão a sessão
   entra em laço.
6. **Campo não confirmado não vira telemetria.** Guarde em `Attributes` e
   escreva `TODO: VERIFY AGAINST DEVICE PROTOCOL`.
7. **Comando sem texto confirmado deve falhar.** Devolva
   `ErrUnsupportedCommand`; quem precisar resolve com override no cadastro do
   dispositivo.
8. **Escreva os testes com os bytes capturados**, cobrindo: quadro válido,
   quadro partido entre leituras, vários quadros numa leitura, checksum
   inválido, IMEI inválido, payload desconhecido.
9. **Registre** em `cmd/server/main.go`. A ordem importa: assinaturas fortes
   primeiro, captura por último.

### Quando o texto do comando diverge

Não é preciso recompilar. O cadastro do dispositivo tem `command_overrides`:

```json
{ "ENGINE_CUT": "RELAY,1#", "ENGINE_RESUME": "RELAY,0#" }
```

O override entra no lugar do texto padrão e aparece igual na auditoria — o que
foi enviado ao aparelho fica registrado como foi enviado.

---

## O que nunca é inventado

- **Velocidade** — armazenada só em km/h; protocolo que reporta em nós é
  convertido na entrada (`KnotsToKmh`), e nunca guardamos o valor em nós.
- **Precisão** — `gps_valid`, `satellites` e `hdop` guardam o que o aparelho
  mandou. A precisão anunciada pelo fabricante não vira número no painel.
- **Bateria** — o GT06 reporta nível relativo (0–6), não tensão. Isso vai para
  `battery_percent` e `battery_voltage` fica nulo.
- **Data/hora** — `gps_timestamp` é o do aparelho, `received_at` é o do
  servidor. Nunca se usa um no lugar do outro: o rastreador pode descarregar
  posições guardadas offline, e o trajeto só se reconstrói direito com os dois.
- **Heartbeat** — não vira posição. Ele traz estado (ACC, bateria, relé) e vai
  para `device_states`; criar uma linha em `positions` com a coordenada
  anterior seria inventar um fix de GPS que o aparelho não enviou.
