// Package migrations embebe los archivos SQL que aplica goose al arrancar.
// 0001 está congelada. Las nuevas usan prefijo por persona: 1xxx_, 2xxx_, 3xxx_.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
