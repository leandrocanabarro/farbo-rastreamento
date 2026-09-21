// Package protocols define o contrato entre o servidor TCP e os dialetos dos
// rastreadores. Nada fora deste pacote (e dos seus subpacotes) deve conhecer
// bytes, checksums ou strings de comando de um fabricante específico.
package protocols

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Confidence classifica o quanto de uma implementação está confirmado pela
// documentação do fabricante. Ver docs/PROTOCOLS.md e a regra §38 do projeto.
type Confidence string

const (
	// Documented: formato conferido contra documentação pública do protocolo.
	Documented Confidence = "DOCUMENTED"
	// Assumed: formato inferido de fontes secundárias, ainda não confirmado
	// contra o firmware real. Todo campo nessa condição carrega um
	// "TODO: VERIFY AGAINST DEVICE PROTOCOL" no código.
	Assumed Confidence = "ASSUMED"
	// Unknown: reconhecimento apenas superficial; o payload é capturado cru.
	Unknown Confidence = "UNKNOWN"
)

// MessageKind identifica o que o rastreador enviou.
type MessageKind string

const (
	KindLogin      MessageKind = "login"
	KindPosition   MessageKind = "position"
	KindHeartbeat  MessageKind = "heartbeat"
	KindStatus     MessageKind = "status"
	KindAlarm      MessageKind = "alarm"
	KindCommandAck MessageKind = "command_ack"
	KindOther      MessageKind = "other"
)

// TrackerMessage é a telemetria normalizada. O domínio só enxerga esta forma.
type TrackerMessage struct {
	IMEI           string
	Timestamp      time.Time
	Latitude       float64
	Longitude      float64
	SpeedKmh       float64
	Heading        float64
	Altitude       *float64
	GPSValid       bool
	Satellites     *int
	ACC            *bool
	BatteryVoltage *float64
	GSMLevel       *int
	RawPayload     string

	// ---- Campos adicionais necessários para o fluxo completo ----

	// Kind diz ao ingestor o que fazer com a mensagem.
	Kind MessageKind
	// Protocol é o nome do adaptador que decodificou o pacote.
	Protocol string
	// Ack são os bytes que devem voltar ao rastreador imediatamente.
	// Vazio significa que o protocolo não exige resposta para este pacote.
	Ack []byte
	// HasLocation distingue um pacote com coordenada nova (posição) de um
	// heartbeat/status, que atualiza estado mas não tem GPS próprio.
	HasLocation bool
	// Alarm é o rótulo do alarme quando Kind == KindAlarm.
	Alarm string
	// CorrelationKey casa uma resposta de comando (KindCommandAck) com o
	// registro em device_commands.
	CorrelationKey uint32
	// Response é o texto devolvido pelo rastreador para um comando.
	Response string
	// RelayOn é o estado do relé de corte informado pelo dispositivo.
	RelayOn *bool
	// BatteryPercent é usado quando o firmware reporta nível relativo em vez
	// de tensão. Não convertemos um no outro (§34).
	BatteryPercent *int
	// HDOP quando o protocolo fornecer.
	HDOP *float64
	// Attributes carrega o que é específico do fabricante sem poluir o domínio.
	Attributes map[string]any
}

// CommandType é o comando canônico, independente de fabricante.
type CommandType string

const (
	CommandEngineCut       CommandType = "ENGINE_CUT"
	CommandEngineResume    CommandType = "ENGINE_RESUME"
	CommandRequestPosition CommandType = "REQUEST_POSITION"
	CommandRequestStatus   CommandType = "REQUEST_STATUS"
	CommandSetInterval     CommandType = "SET_INTERVAL"
	CommandSetHeartbeat    CommandType = "SET_HEARTBEAT"
	CommandSetServer       CommandType = "SET_SERVER"
	CommandReboot          CommandType = "REBOOT"
	CommandCustom          CommandType = "CUSTOM"
)

// AllCommandTypes lista os comandos conhecidos pelo domínio.
func AllCommandTypes() []CommandType {
	return []CommandType{
		CommandEngineCut, CommandEngineResume, CommandRequestPosition,
		CommandRequestStatus, CommandSetInterval, CommandSetHeartbeat,
		CommandSetServer, CommandReboot, CommandCustom,
	}
}

func (c CommandType) Valid() bool {
	for _, known := range AllCommandTypes() {
		if known == c {
			return true
		}
	}
	return false
}

// Command é o pedido normalizado entregue ao encoder do protocolo.
type Command struct {
	Type CommandType
	// UniqueID é o identificador do aparelho (IMEI). Protocolos de texto o
	// repetem dentro do próprio comando, então o encoder precisa dele.
	UniqueID string
	// CorrelationKey viaja dentro do pacote quando o protocolo suporta,
	// permitindo casar o ACK com o comando exato (§16).
	CorrelationKey uint32
	// Password do dispositivo, quando o firmware exigir autenticação.
	Password string
	// Params traz os argumentos do comando (ex.: "seconds", "host", "port").
	Params map[string]string
	// Raw substitui o texto gerado pelo encoder. Usado por CommandCustom e
	// pelos overrides cadastrados no dispositivo.
	Raw string
}

func (c Command) Param(key string) string { return c.Params[key] }

// ErrUnsupportedCommand indica que o protocolo não implementa o comando.
var ErrUnsupportedCommand = errors.New("comando não suportado por este protocolo")

// ErrIncompleteFrame indica que faltam bytes para formar um quadro.
var ErrIncompleteFrame = errors.New("quadro incompleto")

// Framer separa quadros completos do fluxo TCP.
//
// É uma interface separada porque uma leitura de socket não corresponde a um
// pacote: pode trazer meio pacote ou vários (§31). O servidor TCP acumula os
// bytes e pergunta ao protocolo onde cada quadro termina.
type Framer interface {
	// NextFrame examina o início de buf.
	//   - (frame, n, nil)  -> quadro completo; consuma n bytes.
	//   - (nil, 0, ErrIncompleteFrame) -> aguarde mais dados.
	//   - (nil, n, err)    -> lixo/quadro inválido; descarte n bytes e registre o erro.
	NextFrame(buf []byte) (frame []byte, consumed int, err error)
}

// Descriptor descreve o adaptador para a API e para a interface do painel.
type Descriptor struct {
	Name       string        `json:"name"`
	Label      string        `json:"label"`
	Vendor     string        `json:"vendor"`
	Confidence Confidence    `json:"confidence"`
	Commands   []CommandType `json:"commands"`
	Notes      string        `json:"notes"`
}

func (d Descriptor) Supports(c CommandType) bool {
	for _, known := range d.Commands {
		if known == c {
			return true
		}
	}
	return false
}

// TrackerProtocol é o adaptador de um dialeto de rastreador.
type TrackerProtocol interface {
	Framer

	Name() string
	// Detect responde se o início do fluxo pertence a este protocolo.
	Detect(data []byte) bool
	// Parse decodifica UM quadro completo, devolvido antes por NextFrame.
	Parse(data []byte) ([]TrackerMessage, error)
	// EncodeCommand traduz o comando canônico para os bytes do dispositivo.
	EncodeCommand(command Command) ([]byte, error)
	// Describe expõe metadados (comandos suportados, nível de confiança).
	Describe() Descriptor
}

// ProtocolRegistry guarda os adaptadores disponíveis.
type ProtocolRegistry struct {
	protocols []TrackerProtocol
}

func NewRegistry(protocols ...TrackerProtocol) *ProtocolRegistry {
	return &ProtocolRegistry{protocols: protocols}
}

// Detect devolve o primeiro protocolo que reconhece o início do fluxo.
func (r *ProtocolRegistry) Detect(data []byte) (TrackerProtocol, bool) {
	for _, p := range r.protocols {
		if p.Detect(data) {
			return p, true
		}
	}
	return nil, false
}

func (r *ProtocolRegistry) ByName(name string) (TrackerProtocol, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, p := range r.protocols {
		if strings.ToLower(p.Name()) == name {
			return p, true
		}
	}
	return nil, false
}

func (r *ProtocolRegistry) All() []TrackerProtocol { return r.protocols }

func (r *ProtocolRegistry) Descriptors() []Descriptor {
	out := make([]Descriptor, 0, len(r.protocols))
	for _, p := range r.protocols {
		out = append(out, p.Describe())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ValidateIMEI recusa identificadores fora do formato esperado, o que também
// protege contra injeção de comando em protocolos de texto (§27).
func ValidateIMEI(imei string) error {
	imei = strings.TrimSpace(imei)
	if len(imei) < 10 || len(imei) > 20 {
		return fmt.Errorf("IMEI com tamanho inválido: %d", len(imei))
	}
	allZero := true
	for _, r := range imei {
		if r < '0' || r > '9' {
			return fmt.Errorf("IMEI com caractere não numérico")
		}
		if r != '0' {
			allZero = false
		}
	}
	if allZero {
		return fmt.Errorf("IMEI zerado")
	}
	return nil
}

// Ptr é um utilitário para preencher os campos opcionais de TrackerMessage.
func Ptr[T any](v T) *T { return &v }
