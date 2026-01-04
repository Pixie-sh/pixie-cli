package golang

import "embed"

//go:embed templates/*.tmpl
var TemplateFS embed.FS
