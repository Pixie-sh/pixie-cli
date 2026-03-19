package main

import "github.com/pixie-sh/pixie-cli/internal/cli/pixie/app"

// main exists to support `go install github.com/pixie-sh/pixie-cli@latest`.
// The actual CLI bootstrap lives behind the shared runner in `internal/cli/pixie/app`.
func main() {
	app.Run()
}
