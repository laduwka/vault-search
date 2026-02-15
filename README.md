# Vault Search

A fast, secure, local tool for searching HashiCorp Vault secrets by key names and paths. Designed for SREs and DevOps engineers who need to quickly find secrets without exposing values.

## Features

- **Key-Name Search**: Search secret paths and key names without exposing secret values
- **Nested Key Extraction**: Automatically extracts keys from JSON/YAML values in secrets
- **Case-Insensitive Search**: Quick substring matching via `term=` parameter
- **Regex Support**: Full regex control with `regexp=` parameter (user controls case sensitivity)
- **Path Filtering**: Filter results by secret path substring
- **Memory-Only Cache**: No disk persistence - secrets never touch the filesystem
- **Fast Performance**: Pre-built search strings, concurrent fetching, goroutine-limited
- **ReDoS Protection**: 5-second timeout on regex searches

## Installation

### Prerequisites

- Go 1.24+
- HashiCorp Vault (KV v2 secrets engine)
- Valid Vault token with read access to secrets

### Build from Source

```bash
git clone <repository-url>
cd vault-search
go mod tidy
go build -o vault-search .
```

### Using Make

```bash
make build   # Build binary
make run     # Run application
make test    # Run tests
make all     # Run tidy, fmt, vet, test, build
```

### Run with Docker

Build and run using Docker locally:

```bash
# Build the Docker image
docker build -t vault-search:latest .

# Run the container
docker run --rm -p 8080:8080 \
  -e VAULT_TOKEN="your-vault-token" \
  -e VAULT_ADDR="https://your-vault.example.com" \
  vault-search:latest
```

Or using Make:

```bash
# Build Docker image
make docker-build

# Run Docker container
VAULT_TOKEN="your-token" VAULT_ADDR="https://vault.example.com" make docker-run
```

### Pull from GitHub Container Registry

Pre-built images are available on GitHub Container Registry:

```bash
# Pull the latest image
docker pull ghcr.io/laduwka/vault-search:latest

# Pull a specific version
docker pull ghcr.io/laduwka/vault-search:v0.1.0

# Run the container
docker run --rm -p 8080:8080 \
  -e VAULT_TOKEN="your-vault-token" \
  -e VAULT_ADDR="https://your-vault.example.com" \
  ghcr.io/laduwka/vault-search:latest
```

### Verify Docker Image Signature

All Docker images are signed with [Cosign](https://docs.sigstore.dev/cosign/overview/) using keyless signing via Sigstore. You can verify the image signature before running:

#### Install Cosign

```bash
# macOS
brew install cosign

# Linux
go install github.com/sigstore/cosign/v2/cmd/cosign@latest

# Or download from https://github.com/sigstore/cosign/releases
```

#### Verify Image

```bash
# Get the image digest
DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' ghcr.io/laduwka/vault-search:latest | cut -d'@' -f2)

# Verify signature
cosign verify \
  --certificate-identity-regexp="^https://github.com/laduwka/vault-search/.github/workflows/" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/laduwka/vault-search@sha256:$DIGEST
```

#### Verify with Specific Version

```bash
# Pull specific version
docker pull ghcr.io/laduwka/vault-search:v0.1.0

# Verify
cosign verify \
  --certificate-identity-regexp="^https://github.com/laduwka/vault-search/.github/workflows/" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/laduwka/vault-search:v0.1.0
```

Successful verification output:
```
Verification for ghcr.io/laduwka/vault-search@sha256:... --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - The claims were spread
  - The signatures were verified against the specified public key
  - Any certificates were verified against the Fulcio roots.
```

### Run

```bash
# Set required environment variable
export VAULT_TOKEN="your-vault-token"

# Optional: configure other settings
export VAULT_ADDR="https://your-vault.example.com"
export VAULT_MOUNT_POINT="kv"
export LOCAL_SERVER_ADDR="localhost:8080"
export LOG_LEVEL="info"

# Run the server
./vault-search
```

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `VAULT_TOKEN` | *(required)* | Vault authentication token |
| `VAULT_ADDR` | `https://vault.offline.shelopes.com` | Vault server address |
| `VAULT_MOUNT_POINT` | `kv` | KV v2 secrets engine mount point |
| `LOCAL_SERVER_ADDR` | `localhost:8080` | HTTP listen address |
| `MAX_GOROUTINES` | `15` | Concurrency limit for Vault API calls |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LOG_FILE_PATH` | `/tmp/vault_search.log` | Log file path (also logs to stdout) |

## API Reference

### Search Secrets

```
GET /search
```

Search for secrets by key names or paths.

#### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `term` | string | Case-insensitive substring search on path + key names |
| `regexp` | string | Regular expression search (user adds `(?i)` for case-insensitive) |
| `in_path` | string | Filter results to paths containing this substring |
| `sort` | string | Sort results: `asc` or `desc` |
| `show_ui` | boolean | Return Vault UI URLs instead of paths (`true`) |

**Note:** At least one of `term`, `regexp`, or `in_path` is required. `term` and `regexp` are mutually exclusive.

#### Response

```json
{
  "matches": [
    "prod/database/credentials",
    "staging/api/keys"
  ]
}
```

With `show_ui=true`:

```json
{
  "matches": [
    "https://vault.example.com/ui/vault/secrets/kv/show/prod/database/credentials",
    "https://vault.example.com/ui/vault/secrets/kv/show/staging/api/keys"
  ]
}
```

#### Examples

```bash
# Find all secrets containing "password" in any key name or path
curl "http://localhost:8080/search?term=password"

# Find secrets with keys matching regex (case-sensitive)
curl 'http://localhost:8080/search?regexp=^api_'

# Find secrets with keys matching regex (case-insensitive)
curl 'http://localhost:8080/search?regexp=(?i)^api_'

# Find secrets in "prod" path with "db" in key names
curl "http://localhost:8080/search?term=db&in_path=prod"

# Get sorted results with Vault UI links
curl "http://localhost:8080/search?term=api_key&sort=asc&show_ui=true"

# Find secrets containing "credentials" in path only
curl "http://localhost:8080/search?in_path=credentials&sort=desc"
```

### Get Cache Status

```
GET /status
```

Returns cache statistics and rebuild status.

#### Response

```json
{
  "cache_age": "2h 15m 30s",
  "build_duration": "45s",
  "is_rebuilding": false,
  "cache_in_mem_size": "1.2 MB",
  "fetched_secrets": 1500,
  "total_secrets": 1500,
  "total_keys_indexed": 4500,
  "progress_percentage": 100
}
```

#### Fields

| Field | Description |
|-------|-------------|
| `cache_age` | Time since last successful cache build |
| `build_duration` | Duration of last cache build |
| `is_rebuilding` | Whether a rebuild is in progress |
| `cache_in_mem_size` | Estimated memory usage |
| `fetched_secrets` | Number of secrets fetched in current/last build |
| `total_secrets` | Total secrets discovered |
| `total_keys_indexed` | Total key names indexed (including nested) |
| `progress_percentage` | Build progress (0-100) |

### Rebuild Cache

```
POST /rebuild
```

Trigger an asynchronous cache rebuild. Only one rebuild can run at a time.

#### Request Body

```json
{
  "rebuild": "true"
}
```

#### Response

```json
{
  "message": "Cache rebuild started"
}
```

#### Example

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"rebuild": "true"}' \
  "http://localhost:8080/rebuild"
```

## How It Works

### Key Extraction

The tool extracts key names from secrets in three ways:

1. **Top-level keys**: Direct keys in the secret (e.g., `username`, `password`)
2. **Nested JSON keys**: Parses JSON string values like `{"host": "db", "port": 5432}`
3. **Nested YAML keys**: Parses YAML string values like `host: db\nport: 5432`

**Heuristics for nested parsing:**
- JSON: Value starts with `{` or `[`
- YAML: Value contains `:` AND newline character

Parse failures are logged at DEBUG level only.

### Search String Building

Each secret gets a pre-built search string:

```
"path/to/secret key1 key2 nested_key1 nested_key2 "
```

All lowercase for fast case-insensitive substring matching.

### Example

Secret at path `prod/database/credentials`:

```json
{
  "username": "admin",
  "password": "secret123",
  "config": "{\"host\": \"db.example.com\", \"port\": 5432}"
}
```

Extracted keys: `["username", "password", "config", "host", "port"]`

Search string: `"prod/database/credentials username password config host port "`

Searches for `term=port` or `term=PASSWORD` will match this secret.

## Security

### What Gets Cached

| Cached | NOT Cached |
|--------|------------|
| Secret paths | Secret values |
| Key names | Key values |
| Nested key names | Nested key values |

### Security Features

- **Memory-only**: Cache never written to disk
- **No value exposure**: Only key names are searchable
- **ReDoS protection**: 5-second timeout on regex searches
- **Local only**: Designed for localhost use
- **Goroutine limits**: Prevents resource exhaustion

### Recommendations

- Run only on your local machine
- Use a Vault token with minimal required permissions
- Set `LOG_LEVEL=warn` or higher in shared environments
- Rotate Vault tokens regularly

## Performance

### Optimizations

| Feature | Benefit |
|---------|---------|
| Pre-built search strings | No JSON marshaling during search |
| Concurrent secret fetching | Faster cache builds |
| Goroutine semaphore | Controlled Vault API load |
| RWMutex cache | Non-blocking reads during searches |

### Expected Performance

| Secrets | Cache Build | Search Time |
|---------|-------------|-------------|
| 100 | ~2s | <1ms |
| 1,000 | ~10s | <5ms |
| 10,000 | ~60s | <20ms |

*Times vary based on Vault latency and `MAX_GOROUTINES` setting*

## Development

### Project Structure

```
/project/
├── main.go           # Entry point, HTTP server
├── config.go         # Configuration, initialization
├── cache.go          # Cache management
├── handlers.go       # HTTP handlers
├── search.go         # Search logic
├── extract.go        # Key extraction
├── utils.go          # Helper functions
├── main_test.go      # Unit tests
├── go.mod
└── go.sum
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific test
go test -run TestExtractKeysFromValue ./...

# Run with coverage
go test -cover ./...
```

### Code Quality

```bash
# Format code
go fmt ./...

# Static analysis
go vet ./...
```

## Troubleshooting

### Common Issues

#### "Failed to create Vault client"

- Verify `VAULT_ADDR` is correct and accessible
- Check network connectivity to Vault

#### "Access denied for secret"

- Token lacks read permission for that path
- These are logged at WARN level and skipped

#### "Search timeout exceeded"

- Regex is too complex or cache is very large
- Simplify regex or increase timeout in `handlers.go`

#### "Cache rebuild is already in progress"

- Only one rebuild can run at a time
- Wait for current rebuild to complete

### Debug Mode

Enable debug logging for detailed information:

```bash
export LOG_LEVEL=debug
./vault-search
```

Debug logs include:
- Each secret fetched
- JSON/YAML detection in values
- Parse failures for nested content
- Directory traversal details

## License

MIT License

## Contributing

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Run linters: `go fmt ./... && go vet ./...`
6. Submit a pull request
