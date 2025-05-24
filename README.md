# Catapult

Catapult is a command-line tool for synchronizing files with GitHub using device flow authentication. It automatically tracks and syncs all files in your working directory with a GitHub repository.

## Features

- Automatic file tracking - no need to manually add or remove files
- Device flow authentication with GitHub
- Real-time synchronization of file changes
- Conflict detection and resolution
- Progress tracking and status reporting

## Installation

```bash
go install github.com/itcaat/catapult@latest
```

## Usage

### Initialize

Initialize Catapult in your project directory:

```bash
catapult init
```

This will:
1. Set up device flow authentication with GitHub
2. Create a private repository for file synchronization
3. Initialize the local configuration

### Sync

Sync all files in the current directory with GitHub:

```bash
catapult sync
```

This will:
1. Scan the directory for all files
2. Compare local and remote versions
3. Upload new files
4. Update changed files
5. Download remote changes
6. Report any conflicts

### Status

Check the synchronization status of all files:

```bash
catapult status
```

This will show:
- Files that are in sync
- Files with local changes
- Files with remote changes
- Files with conflicts

## Configuration

Catapult uses two configuration files:

1. `config.yaml` - Static configuration:
   ```yaml
   github:
     client_id: "your_client_id"
     scopes: ["repo"]
   storage:
     base_dir: "."
   repository:
     name: "catapult-folder"
   ```

2. `~/.catapult/config.runtime.yaml` - Runtime configuration:
   ```yaml
   github:
     token: "your_github_token"
   ```

The runtime configuration is automatically created and managed by Catapult.

## File Tracking

Catapult automatically tracks all files in your working directory, excluding:
- Hidden files (starting with `.`)
- The `.catapult` directory
- Directories

## License

MIT