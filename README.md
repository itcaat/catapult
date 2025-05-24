# Catapult

Catapult is a command-line tool for managing GitHub repositories and automating common development tasks.

## Features

- GitHub repository management
- Automated workflow execution
- Configuration management
- Secure authentication using GitHub OAuth
- File synchronization with GitHub repositories

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/catapult.git
cd catapult
```

2. Build the application:
```bash
go build -o catapult cmd/catapult/main.go
```

## Configuration

### GitHub OAuth Setup

1. Go to GitHub Settings > Developer Settings > OAuth Apps > New OAuth App
2. Fill in the application details:
   - Application name: Catapult (or your preferred name)
   - Homepage URL: http://localhost
   - Authorization callback URL: http://localhost:9999 (or any other URL, as it's not used in device flow)
3. After creating the app, copy the Client ID

### Configuration Files Structure

Catapult uses two configuration files:

- **Static config:** `config.yaml` (in the project root)
  - Contains only static settings: `clientid`, `scopes`, and repository name.
  - Example:
    ```yaml
    github:
      clientid: "your-client-id-here"  # Replace with your GitHub OAuth App Client ID
      scopes:
        - repo
    repository:
      name: "catapult-folder"
    ```
- **Runtime config:** `~/.catapult/config.runtime.yaml` (created automatically after `init`)
  - Contains dynamic and sensitive data: authentication token, storage paths, state file location.
  - Example (created/updated by the app, not by hand):
    ```yaml
    github:
      token: "gho_xxx..."
    storage:
      basedir: "/Users/youruser/.catapult/files"
      statepath: "/Users/youruser/.catapult/state.json"
      tokenpath: ""
    ```

**How it works:**
- При запуске Catapult сначала читается `config.yaml` из текущей директории.
- Затем (если есть) подмешивается/дополняется runtime-конфиг из домашнего каталога.
- Все изменения (например, токен) сохраняются только в runtime-конфиг.
- Никогда не храните токен в проектном `config.yaml` — он всегда будет только в `~/.catapult/config.runtime.yaml`.

### Local Configuration (Quick Start)

1. Создайте `config.yaml` в корне проекта (см. выше).
2. Запустите `./catapult init` — будет создан runtime-конфиг с токеном и путями.

## Usage

### Initialization

Run the initialization command:
```bash
./catapult init
```

The tool will:
1. Request a device code from GitHub
2. Display a URL and code for you to enter
3. Open your browser to complete the authorization
4. Wait for you to enter the code and confirm the authorization
5. Store the authentication token securely

### File Synchronization

To synchronize files with GitHub:

1. First, ensure you have initialized the tool with `./catapult init`
2. Add files to track:
```bash
./catapult add <file>
```
3. Sync files with GitHub:
```bash
./catapult sync
```

The sync process will:
- Track changes in your local files
- Upload changes to GitHub
- Maintain synchronization state in `~/.catapult/state.json`

### Available Commands

- `init`: Initialize the tool with GitHub authentication
- `add <file>`: Add a file to track for synchronization
- `sync`: Synchronize tracked files with GitHub
- `help`: Display help information

## Development

### Project Structure

```
.
├── cmd/
│   └── catapult/        # Main application entry point
├── internal/
│   ├── auth/           # Authentication handling
│   ├── config/         # Configuration management
│   └── storage/        # Secure storage implementation
├── config.yaml         # Configuration file
└── README.md
```

### Building

```bash
go build -o catapult cmd/catapult/main.go
```

## Security

- Authentication tokens are stored securely in the user's home directory
- OAuth device flow is used for authentication, which is more secure than traditional OAuth flows
- No sensitive data is stored in the configuration file
- State file is stored in the user's home directory with appropriate permissions

## Troubleshooting

### Common Issues

1. "Failed to load state" error:
   - Ensure you've run `./catapult init` first
   - Check if the `~/.catapult` directory exists
   - Verify you have write permissions in your home directory

2. Authentication issues:
   - Verify your Client ID in `config.yaml`
   - Ensure you've completed the GitHub OAuth App setup
   - Check if your token is still valid

## License

MIT License