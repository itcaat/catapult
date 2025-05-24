# Catapult

A console application for file management and synchronization with GitHub using device flow authentication.

## Features

- **Device Flow Authentication**: Secure OAuth authentication without storing credentials
- **Bidirectional Sync**: Sync files between local directory and GitHub repository
- **Efficient API Usage**: Single API call to check all file statuses
- **Git SHA Comparison**: Uses Git SHA-1 for efficient file comparison
- **Conflict Detection**: Detects and handles conflicts between local and remote changes
- **Cross-platform**: Available for Linux, macOS, and Windows

## Installation

### Download Pre-built Binaries

Download the latest release from the [Releases page](https://github.com/itcaat/catapult/releases).

Available platforms:
- Linux (amd64, arm64)
- macOS (amd64, arm64, universal)
- Windows (amd64)

### Build from Source

```bash
git clone https://github.com/itcaat/catapult.git
cd catapult
go build -o catapult ./cmd/catapult
```

## Usage

### Initialize

Set up authentication and repository:

```bash
./catapult init
```

This will:
1. Start the OAuth device flow
2. Open your browser for GitHub authentication
3. Create the repository if it doesn't exist
4. Set up local configuration

### Check Status

View the synchronization status of all files:

```bash
./catapult status
```

Status indicators:
- **Synced**: File is identical locally and remotely
- **Local-only**: File exists only locally (needs to be uploaded)
- **Remote-only**: File exists only remotely (needs to be downloaded)
- **Modified locally**: Local file has changes (needs to be synced)
- **Modified in repository**: Remote file has changes (needs to be pulled)
- **Conflict**: Both local and remote have changes

### Sync Files

Synchronize all files with the repository:

```bash
./catapult sync
```

This will:
- Upload new local files
- Download new remote files
- Update changed files
- Handle conflicts (currently uses local version)

## Configuration

Configuration is stored in `config.yaml`:

```yaml
github:
  client_id: "Ov23liOGFaHlgPjzm5B3"
  scopes: ["repo"]
  token: "your_access_token"
repository:
  name: "catapult-folder"
storage:
  base_dir: "./catapult-files"
  state_path: "./catapult-files/.catapult-state.json"
```

## Development

### Prerequisites

- Go 1.21 or later
- Git

### Building

```bash
go build -o catapult ./cmd/catapult
```

### Testing

```bash
go test ./...
```

### Release Process

This project uses GitHub Actions and GoReleaser for automated releases:

1. **Create a tag**: `git tag v1.0.0 && git push origin v1.0.0`
2. **GitHub Actions** automatically:
   - Runs tests
   - Builds binaries for all platforms
   - Creates GitHub release
   - Uploads artifacts

### Local Release Testing

Test the release process locally:

```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser/v2@latest

# Test build
goreleaser build --snapshot --clean

# Check configuration
goreleaser check
```

## Architecture

- **Authentication**: OAuth device flow for secure GitHub access
- **Storage**: Local file tracking with JSON state file
- **Sync Logic**: Three-way comparison using Git SHA-1 hashes
- **API Efficiency**: Batch operations to minimize GitHub API calls

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `go test ./...`
6. Submit a pull request

## License

MIT License - see LICENSE file for details.