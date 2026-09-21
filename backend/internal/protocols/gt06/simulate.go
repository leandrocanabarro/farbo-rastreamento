package gt06

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// Este arquivo expõe a montagem de quadros do lado do RASTREADOR.
//
// Existe para que o simulador (cmd/tksim) e os testes de integração falem o
// protocolo real, gerado pelo mesmo código que o decodificador entende — em
// vez de manter uma segunda cópia do layout de bytes que pode divergir.

// DeviceStatus é o estado que o rastreador reporta nos pacotes.
type DeviceStatus struct {
	// ACC ligado (ignição).
	ACC bool
	// RelayOn indica corte de óleo/energia acionado.
	RelayOn bool
	// Charging indica carregamento da bateria interna.
	Charging bool
	// BatteryLevel de 0 a 6, como o GT06 reporta.
	BatteryLevel byte
	// GSMLevel de 0 a 4.
	GSMLevel byte
}

// TerminalInfoByte monta o byte de estado do terminal.
func (s DeviceStatus) TerminalInfoByte() byte {
	var info byte = 0x40 // GPS rastreando
	if s.RelayOn {
		info |= 0x80
	}
	if s.Charging {
		info |= 0x04
	}
	if s.ACC {
		info |= 0x02
	}
	info |= 0x01 // armado
	return info
}

// Fix é uma posição de GPS do ponto de vista do aparelho.
type Fix struct {
	Time       time.Time
	Latitude   float64
	Longitude  float64
	SpeedKmh   float64
	CourseDeg  float64
	Satellites int
	Valid      bool
}

// BuildLoginFrame monta o pacote 0x01 com o IMEI em BCD.
func BuildLoginFrame(imei string, serial uint16) ([]byte, error) {
	if len(imei) > 16 {
		return nil, fmt.Errorf("IMEI longo demais: %q", imei)
	}
	padded := fmt.Sprintf("%016s", imei)

	content := make([]byte, 8)
	for i := range 8 {
		high, err := digit(padded[i*2])
		if err != nil {
			return nil, err
		}
		low, err := digit(padded[i*2+1])
		if err != nil {
			return nil, err
		}
		content[i] = high<<4 | low
	}
	return buildFrame(msgLogin, content, serial), nil
}

func digit(c byte) (byte, error) {
	if c < '0' || c > '9' {
		return 0, fmt.Errorf("IMEI com caractere não numérico: %q", string(c))
	}
	return c - '0', nil
}

// BuildHeartbeatFrame monta o pacote 0x13.
func BuildHeartbeatFrame(status DeviceStatus, serial uint16) []byte {
	content := []byte{
		status.TerminalInfoByte(),
		clampLevel(status.BatteryLevel, 6),
		clampLevel(status.GSMLevel, 4),
		0x00, 0x01, // alarme/idioma
	}
	return buildFrame(msgHeartbeat, content, serial)
}

// BuildPositionFrame monta o pacote 0x2D (GPS + LBS + estado).
func BuildPositionFrame(fix Fix, status DeviceStatus, serial uint16) []byte {
	content := encodeGPSBlock(fix)
	content = append(content, encodeLBSBlock()...)
	content = append(content,
		status.TerminalInfoByte(),
		clampLevel(status.BatteryLevel, 6),
		clampLevel(status.GSMLevel, 4),
	)
	return buildFrame(msgGPSLBSStatus, content, serial)
}

// BuildAlarmFrame monta o pacote 0x16 com um código de alarme.
func BuildAlarmFrame(fix Fix, status DeviceStatus, alarmCode byte, serial uint16) []byte {
	content := encodeGPSBlock(fix)
	content = append(content, encodeLBSBlock()...)
	content = append(content,
		status.TerminalInfoByte(),
		clampLevel(status.BatteryLevel, 6),
		clampLevel(status.GSMLevel, 4),
		alarmCode,
		0x01, // idioma
	)
	return buildFrame(msgAlarmGPS, content, serial)
}

// BuildCommandReply monta o pacote 0x15, a resposta do aparelho a um comando.
// O serverFlag precisa ser o mesmo recebido, senão o servidor não consegue
// casar a resposta com o comando (§16).
func BuildCommandReply(serverFlag uint32, text string, serial uint16) []byte {
	content := make([]byte, 0, 7+len(text))
	content = append(content, byte(len(text)+4))
	content = binary.BigEndian.AppendUint32(content, serverFlag)
	content = append(content, text...)
	content = append(content, 0x00, 0x02)
	return buildFrame(msgStringInfo, content, serial)
}

// DecodeServerCommand lê um pacote 0x80 vindo do servidor.
func DecodeServerCommand(frame []byte) (serverFlag uint32, text string, err error) {
	pkt, err := decodeFrame(frame)
	if err != nil {
		return 0, "", err
	}
	if pkt.typ != msgCommand {
		return 0, "", fmt.Errorf("pacote 0x%02X não é comando", pkt.typ)
	}
	if len(pkt.content) < 5 {
		return 0, "", fmt.Errorf("comando curto demais")
	}

	length := int(pkt.content[0])
	if length < 4 || 1+length > len(pkt.content) {
		length = len(pkt.content) - 1
	}
	serverFlag = binary.BigEndian.Uint32(pkt.content[1:5])
	return serverFlag, string(pkt.content[5 : 1+length]), nil
}

func encodeGPSBlock(fix Fix) []byte {
	ts := fix.Time.UTC()
	satellites := byte(fix.Satellites)
	if satellites > 15 {
		satellites = 15
	}

	out := []byte{
		byte(ts.Year() - 2000), byte(ts.Month()), byte(ts.Day()),
		byte(ts.Hour()), byte(ts.Minute()), byte(ts.Second()),
		0xC0 | satellites,
	}
	out = binary.BigEndian.AppendUint32(out, uint32(math.Round(math.Abs(fix.Latitude)*1800000)))
	out = binary.BigEndian.AppendUint32(out, uint32(math.Round(math.Abs(fix.Longitude)*1800000)))

	speed := fix.SpeedKmh
	if speed < 0 {
		speed = 0
	}
	if speed > 255 {
		speed = 255
	}
	out = append(out, byte(math.Round(speed)))

	flags := uint16(math.Round(fix.CourseDeg)) & 0x03FF
	if fix.Valid {
		flags |= 0x1000
	}
	if fix.Latitude >= 0 {
		flags |= 0x0400 // hemisfério norte
	}
	if fix.Longitude < 0 {
		flags |= 0x0800 // longitude oeste
	}
	return binary.BigEndian.AppendUint16(out, flags)
}

// encodeLBSBlock devolve um bloco LBS fixo (MCC 724 Brasil, MNC 06).
func encodeLBSBlock() []byte {
	out := binary.BigEndian.AppendUint16(nil, 724)
	out = append(out, 6)
	out = binary.BigEndian.AppendUint16(out, 0x1234)
	return append(out, 0x00, 0xAB, 0xCD)
}

func clampLevel(value, maximum byte) byte {
	if value > maximum {
		return maximum
	}
	return value
}
