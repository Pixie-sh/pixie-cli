# Pixie CLI

A CLI tool for generating complete projects and boilerplate code for Go backend, Angular frontend, and Expo mobile applications.

## Installation

```bash
go install github.com/pixie-sh/pixie-cli/cmd/cli/cli_pixie@latest
```

Or build from source:

```bash
git clone https://github.com/pixie-sh/pixie-cli.git
cd pixie-cli
go build -o pixie ./cmd/cli/cli_pixie/
```

## Commands

### `pixie init` — Project Initialization

Scaffold a complete project for your chosen technology stack.

```bash
# Go backend with microservices architecture
pixie init golang --name my-backend --module github.com/company/my-backend

# Angular frontend with standalone components
pixie init angular --name my-frontend

# Expo/React Native mobile application
pixie init expo --name my-mobile-app
```

The Go backend generator creates a full project structure including authentication, notifications, domain layers, infrastructure, CI/CD workflows, and an optional CLI tool.

```bash
# Customize the project microservice name
pixie init golang --name my-backend --module github.com/company/my-backend --project-ms orders

# Generate only specific microservices
pixie init golang --name my-backend --module github.com/company/my-backend --microservices authentication,notifications

# Skip CLI tool generation
pixie init golang --name my-backend --module github.com/company/my-backend --with-cli=false
```

---

### `pixie generate` — Code Generation & Analysis

Generate boilerplate code and analyze existing codebases within Go projects.

#### Scaffold Commands

**Generate a microservice:**

```bash
# Basic microservice with default features (database, metrics, auth)
pixie generate microservice --name user_management --domain users

# With specific features and custom ports
pixie generate microservice --name payment_service --domain payments \
  --features database,auth,cache --port 8081 --metrics-port 9091

# Minimal template (metrics only)
pixie generate microservice --name health_check --domain health --template minimal

# Full template (all features enabled)
pixie generate microservice --name main_service --domain core --template full
```

Available features: `database`, `metrics`, `auth`, `cache`, `tokens`, `events`, `notifications`, `backoffice`, `validation`, `adapters`, `apis`.

Feature dependencies are resolved automatically (e.g. `auth` enables `tokens`, `backoffice` enables `auth`).

**Generate a domain** (business logic layer without HTTP controllers):

```bash
pixie generate domain --domain orders --features database,auth
```

**Generate an entity** with migration in an existing domain:

```bash
# Basic entity
pixie generate entity --domain catalog --name product

# User-scoped entity (adds UserID field)
pixie generate entity --domain orders --name invoice --features auth
```

**Generate a service** in an existing domain:

```bash
pixie generate service --domain users --name email
```

**Generate a repository** in an existing domain:

```bash
# Uses domain name as default entity
pixie generate repository --domain orders --name payment

# Reference a specific entity
pixie generate repository --domain orders --name payment --entity transaction
```

#### OpenAPI Commands

Analyze existing Go source code to extract endpoint information and generate OpenAPI specifications.

```bash
# Generate OpenAPI spec from all microservices
pixie generate openapi-spec --output api-spec.yaml --verbose

# Generate for specific microservices only
pixie generate openapi-spec --ms user_management --ms payment_service -o spec.yaml

# Output as JSON
pixie generate openapi-spec --format json --output api-spec.json

# Extract endpoints as JSON
pixie generate extract-endpoints --verbose

# Extract from specific microservices, output to file
pixie generate extract-endpoints --ms payments --ms notifications -o endpoints.json

# Extract endpoints as YAML or table format
pixie generate extract-endpoints --format yaml
pixie generate extract-endpoints --format table
```

---

### Embedding in Other CLIs

The `generate` command group is exported as a public Go API, allowing you to embed it directly into your own Cobra-based CLI.

```go
import "github.com/pixie-sh/pixie-cli/pkg/commands"

// Add pixie's generate command to your root command
rootCmd.AddCommand(commands.GenerateCmd())
```

This exposes the full `generate` subcommand tree (`microservice`, `domain`, `entity`, `service`, `repository`, `openapi-spec`, `extract-endpoints`) within your own CLI tool.

---

## Configuration

Directory conventions and naming patterns can be customized via `.pixie.yaml` or `pixie.yaml` in the project root.

```yaml
generate:
  microservice_dir: "internal/ms"
  domain_dir: "internal/domain"
  models_dir: "pkg/models"
  configs_dir: "misc/configs"
  cmd_dir: "cmd/ms"
  microservice_prefix: "ms_"
  business_layer_suffix: "_business_layer"
  openapi_title: "My Project API"
  openapi_servers:
    - "https://api.example.com"
  oauth_authorize: "https://auth.example.com/authorize"
  oauth_token: "https://auth.example.com/token"
  module_name: "github.com/company/my-project"  # auto-detected from go.mod if omitted
```

All fields are optional. Defaults are used when no config file is present or when fields are omitted.

## Shell Completion

```bash
# Bash
pixie completion bash > /etc/bash_completion.d/pixie

# Zsh
pixie completion zsh > "${fpath[1]}/_pixie"

# Fish
pixie completion fish > ~/.config/fish/completions/pixie.fish
```

## License

See [LICENSE](LICENSE) for details.
