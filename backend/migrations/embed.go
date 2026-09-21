// Package migrations embute os arquivos SQL aplicados na inicialização.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
