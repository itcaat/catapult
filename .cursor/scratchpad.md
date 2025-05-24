# Catapult - GitHub File Sync Console Application

## Project Information
- Name: Catapult
- Repository: https://github.com/itcaat/catapult
- Description: A console application for file management and synchronization with GitHub using device flow authentication

## Background and Motivation
Catapult — CLI-инструмент для синхронизации файлов с GitHub, с поддержкой двусторонней синхронизации и разрешения конфликтов. Следующий этап — улучшение UX и CLI для повышения удобства пользователя.

## Key Challenges and Analysis
1. GitHub Authentication
   - Implementing device flow authentication
     * Using GitHub OAuth device flow API
     * Handling user code display and verification
     * Managing polling for token
   - Handling token storage and refresh
     * Secure storage of access tokens
     * Token refresh before expiration
     * Handling token revocation
   - Managing user sessions
     * Session persistence
     * Multiple repository support
     * User profile management

2. File Management
   - Local file system operations
     * File watching for changes
     * Efficient file reading/writing
     * Handling large files
   - File change detection
     * Using file hashing for change detection
     * Tracking file metadata
     * Handling file moves/renames
   - Conflict resolution
     * Automatic conflict detection
     * Merge strategies
     * User intervention for complex conflicts

3. GitHub Integration
   - Repository operations
     * Repository creation and initialization
     * Branch management
     * Commit and push operations
   - File synchronization
     * Efficient diff calculation
     * Batch operations
     * Progress tracking
   - Error handling
     * Network retry mechanisms
     * Rate limit handling
     * Error recovery

- Необходимо информировать пользователя о ходе синхронизации, статусах файлов и возникающих конфликтах.
- Требуется реализовать удобный механизм ручного разрешения конфликтов.
- Для прозрачности работы — добавить просмотр истории изменений файлов.

## Technical Stack
1. Core Technologies
   - Go 1.21+ (latest stable)
   - Cobra for CLI
   - Viper for configuration
   - go-git for Git operations
   - github.com/google/go-github for GitHub API

2. Project Structure
```
.
├── cmd/
│   └── catapult/
│       └── main.go
├── internal/
│   ├── auth/
│   │   ├── device_flow.go
│   │   └── token_manager.go
│   ├── config/
│   │   └── config.go
│   ├── git/
│   │   ├── operations.go
│   │   └── sync.go
│   └── storage/
│       ├── file_manager.go
│       └── metadata.go
├── pkg/
│   ├── cli/
│   │   └── commands.go
│   └── utils/
│       └── helpers.go
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```

## User Stories

### Story 1: GitHub Authentication
As a user, I want to authenticate with GitHub using device flow
So that I can securely access my GitHub account without sharing my credentials

Acceptance Criteria:
- User runs the application
- If not authenticated, application initiates device flow
- User sees a code to enter on GitHub
- After successful authentication, token is securely stored
- Application remembers authentication state

### Story 2: Repository Management
As a user, I want to work with a dedicated repository for file synchronization
So that I can store and manage my files in a centralized location

Acceptance Criteria:
- Application checks for existence of 'catapult-folder' repository
- If repository doesn't exist, creates it automatically
- If repository exists, connects to it
- Repository is properly initialized with .gitignore and README
- User can start working with the repository immediately

## Implementation Plan

### Phase 1: Authentication Flow
1. Device Flow Implementation
   - [ ] Create device flow client
   - [ ] Implement user code display
   - [ ] Add polling mechanism
   - [ ] Handle token storage

2. Token Management
   - [ ] Implement secure token storage
   - [ ] Add token validation
   - [ ] Create token refresh mechanism

### Phase 2: Repository Management
1. Repository Check
   - [ ] Implement repository existence check
   - [ ] Add repository creation if not exists
   - [ ] Handle repository connection

2. Repository Setup
   - [ ] Create initial repository structure
   - [ ] Add .gitignore
   - [ ] Create README
   - [ ] Initialize Git repository

## Technical Implementation Details

### Authentication Flow
```go
// internal/auth/device_flow.go
type DeviceFlow struct {
    client *github.Client
    config *Config
}

func (df *DeviceFlow) Initiate() (*Token, error) {
    // 1. Request device code
    // 2. Display user code
    // 3. Poll for token
    // 4. Store token
}

// internal/auth/token_manager.go
type TokenManager struct {
    storage Storage
}

func (tm *TokenManager) Store(token *Token) error {
    // Store token securely
}

func (tm *TokenManager) Get() (*Token, error) {
    // Retrieve stored token
}
```

### Repository Management
```go
// internal/git/repository.go
type Repository struct {
    client *github.Client
    token  *Token
}

func (r *Repository) EnsureExists() error {
    // 1. Check if repository exists
    // 2. Create if not exists
    // 3. Initialize repository
}

func (r *Repository) Initialize() error {
    // 1. Create .gitignore
    // 2. Create README
    // 3. Initialize Git
}
```

## Project Status Board
- [x] Project initialization
- [x] Basic project structure
- [x] GitHub authentication
- [x] Repository management
- [x] File management
- [x] GitHub integration
- [x] CLI interface
- [ ] Testing
- [ ] Documentation
- [x] Detailed user messages for sync and conflict
- [ ] Command for manual conflict resolution
- [ ] Command for viewing file change history

## Executor's Feedback or Assistance Requests
*Detailed user feedback for sync/pull is implemented:*
- For each file, the status is shown (synced, updated, pulled from repository, conflict, error).
- In case of conflict, paths to local and remote versions are displayed.
- At the end, a summary for all files is shown.
- CLI commands sync and pull now use this output.

*Previously implemented:*
- GitHub device flow authentication
- Automatic repository initialization and check
- File management and state tracking
- Bidirectional sync with GitHub and conflict handling
- Basic CLI interface for all main operations

Ready to proceed to the next task: manual conflict resolution command.

## Lessons
*This section will be updated with learnings and best practices*

## Success Criteria
1. User can successfully authenticate with GitHub using device flow
2. Application automatically creates or connects to 'catapult-folder' repository
3. Repository is properly initialized with necessary files
4. Authentication state is properly maintained between sessions
5. All operations are performed securely
6. User receives clear feedback about the process

## Testing Strategy
1. Unit Tests
   - Test each package independently
   - Mock external dependencies
   - Achieve >80% code coverage

2. Integration Tests
   - Test GitHub API integration
   - Test file system operations
   - Test synchronization logic

3. End-to-End Tests
   - Test complete workflows
   - Test error scenarios
   - Test user interactions

## Security Considerations
1. Token Security
   - Encrypt stored tokens
   - Implement secure key storage
   - Handle token revocation

2. File Security
   - Validate file operations
   - Implement access control
   - Handle sensitive files

3. Network Security
   - Use HTTPS for all connections
   - Implement rate limiting
   - Handle network errors

## Repository Setup
1. Initial Repository Configuration
   - [ ] Set up GitHub repository at https://github.com/itcaat/catapult
   - [ ] Configure branch protection rules
   - [ ] Set up GitHub Actions for CI/CD
   - [ ] Add issue templates
   - [ ] Create pull request template

2. Documentation
   - [ ] Create comprehensive README.md
   - [ ] Add CONTRIBUTING.md
   - [ ] Add LICENSE file
   - [ ] Create documentation for API and usage 