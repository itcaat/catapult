# Catapult

Catapult is a command-line tool for managing GitHub repositories and automating common development tasks.

## Features

- GitHub repository management
- Automated workflow execution
- Configuration management
- Secure authentication using GitHub OAuth

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

### Local Configuration

1. Create a `config.yaml` file in the project root:
```yaml
github:
  clientid: "your-client-id-here"  # Replace with your GitHub OAuth App Client ID
  scopes:
    - repo
storage:
  path: "~/.catapult"
```

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

### Available Commands

- `init`: Initialize the tool with GitHub authentication
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

## License

MIT License