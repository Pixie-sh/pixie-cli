package version

// Version and build information (set via ldflags at build time)
// Example: go build -ldflags="-X 'github.com/pixie-sh/pixie-cli/internal/version.Version=v1.0.0'"
var (
	// Version is the semantic version (from git tags)
	Version = "dev"

	// Commit is the git commit hash (short form)
	Commit = "unknown"
)

// Info returns formatted version information
func Info() string {
	return Version + " (" + Commit + ")"
}
