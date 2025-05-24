# Catapult

A production-ready console application for automatic file management and synchronization with GitHub using device flow authentication.

## Features

### Core Functionality
- **Device Flow Authentication**: Secure OAuth authentication without storing credentials
- **Bidirectional Sync**: Intelligent sync between local directory and GitHub repository
- **Efficient API Usage**: Optimized GitHub API calls with batch operations
- **Git SHA Comparison**: Uses Git SHA-1 for efficient file change detection
- **Conflict Detection**: Smart conflict resolution with user intervention options
- **Cross-platform**: Available for Linux, macOS, and Windows

### Automatic Synchronization 🆕
- **File Watcher**: Real-time monitoring of local file changes with smart debouncing
- **Background Sync**: Automatic synchronization without manual intervention
- **Network Detection**: Intelligent connectivity monitoring with retry mechanisms
- **Offline Queue**: Persistent storage of sync operations when network is unavailable
- **System Autostart**: Automatic startup after system reboot (macOS/Linux)

### Network Resilience 🆕
- **Multi-endpoint Testing**: Connectivity validation against GitHub and fallback endpoints
- **Exponential Backoff**: Smart retry logic with increasing delays (1s→30s max)
- **Offline Operation**: Continues working without network, syncs when available
- **Queue Management**: Automatic cleanup and capacity management for offline operations

### System Integration 🆕
- **macOS LaunchAgent**: Native system service with network state monitoring
- **Linux systemd**: User-level service with proper dependency management
- **Service Management**: Complete CLI for install/uninstall/start/stop/restart/status/logs
- **Production Ready**: Robust error handling and graceful failure recovery

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

## Quick Start

### 1. Initialize

Set up authentication and repository:

```bash
./catapult init
```

This will:
1. Start the OAuth device flow
2. Open your browser for GitHub authentication
3. Create the repository if it doesn't exist
4. Set up local configuration

### 2. Manual Sync

Synchronize files manually:

```bash
./catapult sync
```

### 3. Automatic Sync (Recommended)

Start automatic file monitoring:

```bash
./catapult sync --watch
```

### 4. System Autostart (Production)

Install as system service for automatic startup:

```bash
# Install autostart service
./catapult service install

# Check service status
./catapult service status

# View service logs
./catapult service logs
```

## Usage

### Basic Commands

#### Check Status
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

#### Manual Sync
Synchronize all files with the repository:

```bash
./catapult sync
```

#### Automatic Sync
Start file watching for automatic synchronization:

```bash
./catapult sync --watch
```

Features:
- Real-time file change detection
- Smart debouncing (groups rapid changes)
- Automatic conflict resolution
- Background operation with progress indicators

### Service Management 🆕

#### Install System Service
```bash
./catapult service install
```
Installs catapult as system service for automatic startup after reboot.

#### Service Control
```bash
./catapult service start      # Start service manually
./catapult service stop       # Stop service
./catapult service restart    # Restart service
./catapult service status     # Check service status
./catapult service logs -n 20 # View last 20 log lines
```

#### Uninstall Service
```bash
./catapult service uninstall
```
Completely removes system service and autostart configuration.

### Platform-Specific Service Details

#### macOS
- **Service Type**: LaunchAgent (user-level)
- **Location**: `~/Library/LaunchAgents/com.itcaat.catapult.plist`
- **Logs**: `~/Library/Logs/catapult.log`
- **Features**: Network state monitoring, automatic restart on failure

#### Linux
- **Service Type**: systemd user service
- **Location**: `~/.config/systemd/user/catapult.service`
- **Logs**: Available via `journalctl --user -u catapult`
- **Features**: Network dependency management, restart on failure

#### Windows
- **Status**: Placeholder implementation (future development)
- **Alternative**: Manual startup or Task Scheduler

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

### Auto-Sync Configuration (Future)
```yaml
auto_sync:
  enabled: true
  watch_local_changes: true
  check_remote_interval: "5m"
  debounce_delay: "2s"
  retry_attempts: 3
  offline_queue: true
  max_queue_size: 1000
```

## Architecture

### Core Components
- **Authentication**: OAuth device flow for secure GitHub access
- **Storage**: Local file tracking with JSON state management
- **Sync Engine**: Three-way comparison using Git SHA-1 hashes
- **API Optimization**: Efficient batch operations to minimize rate limits

### Auto-Sync System 🆕
- **File Watcher**: `fsnotify`-based cross-platform file monitoring
- **Debouncer**: Smart grouping of rapid file changes (2s window)
- **Network Detector**: Multi-endpoint connectivity testing
- **Offline Queue**: Persistent JSON storage for failed operations
- **Service Manager**: Cross-platform system service integration

### Network Resilience
- **Connectivity Endpoints**: GitHub API, GitHub.com, Google.com
- **Retry Strategy**: Exponential backoff (1s, 2s, 4s, 8s, 16s, 30s max)
- **Timeout Handling**: Context-aware operation timeouts
- **Graceful Degradation**: Continues working offline, syncs when available

## Development

### Prerequisites

- Go 1.21 or later
- Git

### Building

```bash
go build -o catapult ./cmd/catapult
```

### Testing

Run all tests:
```bash
go test ./...
```

Run specific package tests:
```bash
go test ./internal/autosync    # Auto-sync functionality
go test ./internal/network     # Network detection
go test ./internal/service     # System service management
go test ./internal/sync        # Core sync logic
```

### Project Structure

```
.
├── cmd/catapult/           # Application entry point
├── internal/
│   ├── auth/              # GitHub device flow authentication
│   ├── autosync/          # Automatic sync with file watcher
│   ├── cmd/               # CLI command definitions
│   ├── config/            # Configuration management
│   ├── network/           # Network connectivity detection
│   ├── service/           # System service management
│   ├── status/            # File status reporting
│   ├── storage/           # Local file state management
│   └── sync/              # Core synchronization logic
├── go.mod
├── go.sum
└── README.md
```

### Release Process

This project uses GitHub Actions and GoReleaser for automated releases:

1. **Create a tag**: `git tag v1.0.0 && git push origin v1.0.0`
2. **GitHub Actions** automatically:
   - Runs all tests (31/31 test cases)
   - Builds binaries for all platforms
   - Creates GitHub release with artifacts
   - Validates cross-platform compatibility

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

## Troubleshooting

### Service Issues

#### macOS
```bash
# Check if LaunchAgent is loaded
launchctl list | grep catapult

# Manually load/unload
launchctl load ~/Library/LaunchAgents/com.itcaat.catapult.plist
launchctl unload ~/Library/LaunchAgents/com.itcaat.catapult.plist

# Check logs
tail -f ~/Library/Logs/catapult.log
```

#### Linux
```bash
# Check systemd service status
systemctl --user status catapult

# View detailed logs
journalctl --user -u catapult -f

# Restart service manually
systemctl --user restart catapult
```

### Network Issues
- **Offline Mode**: Operations are queued and will sync when network returns
- **API Rate Limits**: Built-in retry with exponential backoff
- **Connectivity Problems**: Multi-endpoint testing ensures robust detection

### File Sync Issues
- **Conflicts**: Currently resolved using local version (future: interactive resolution)
- **Large Files**: Efficient streaming with progress indicators
- **Permissions**: Ensure read/write access to sync directory

## Performance

- **File Watching**: Minimal CPU usage with smart debouncing
- **Network Usage**: Optimized API calls, only sync changed files
- **Memory Usage**: Efficient queue management with automatic cleanup
- **Startup Time**: Fast service startup with network readiness detection

## Security

- **Token Storage**: Secure local storage of GitHub access tokens
- **No Credentials**: Device flow eliminates credential handling
- **Scope Limitation**: Minimal required GitHub permissions (repo access only)
- **User-Level Services**: No system-level privileges required

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Run all tests: `go test ./...`
6. Update documentation as needed
7. Submit a pull request

### Development Guidelines

- Follow Go best practices and idioms
- Maintain test coverage >80%
- Add comprehensive error handling
- Document public APIs
- Test cross-platform compatibility

## Roadmap

### Upcoming Features
- [ ] Interactive conflict resolution
- [ ] Windows service implementation
- [ ] Configuration file auto-sync settings
- [ ] Multiple repository support
- [ ] File filtering and ignore patterns
- [ ] Sync performance metrics
- [ ] Web dashboard for sync status

### Completed Features ✅
- [x] CLI architecture refactoring
- [x] Auto-sync with file watcher
- [x] Network detection and offline queue
- [x] Cross-platform system service integration
- [x] Comprehensive testing suite
- [x] Production-ready error handling

## License

MIT License - see LICENSE file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/itcaat/catapult/issues)
- **Discussions**: [GitHub Discussions](https://github.com/itcaat/catapult/discussions)
- **Documentation**: This README and inline code documentation