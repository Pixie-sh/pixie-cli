package scaffold

import "embed"

//go:embed templates/*.tmpl
var Templates embed.FS
