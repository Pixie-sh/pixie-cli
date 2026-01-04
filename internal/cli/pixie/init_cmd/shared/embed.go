package shared

import "embed"

//go:embed templates/github_actions/*.tmpl
var GitHubActionsTemplates embed.FS
