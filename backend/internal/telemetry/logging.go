// Package telemetry concentra logs estruturados, métricas Prometheus e tracing OTel.
package telemetry

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger cria o logger estruturado da aplicação.
func NewLogger(level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	if strings.EqualFold(format, "text") {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// maskIMEI controla o mascaramento global do IMEI nos logs (§28).
var maskIMEI = true

func SetMaskIMEI(mask bool) { maskIMEI = mask }

// IMEI devolve o identificador pronto para log, mascarado quando configurado.
// Ex.: 869247061234567 -> 869247******567
func IMEI(imei string) string {
	if !maskIMEI || len(imei) < 9 {
		if !maskIMEI {
			return imei
		}
		return strings.Repeat("*", len(imei))
	}
	head := imei[:6]
	tail := imei[len(imei)-3:]
	return head + strings.Repeat("*", len(imei)-9) + tail
}
