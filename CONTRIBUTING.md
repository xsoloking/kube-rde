# Contributing to KubeRDE

Thank you for your interest in contributing to KubeRDE! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How to Contribute](#how-to-contribute)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Community](#community)

## Code of Conduct

This project adheres to a Code of Conduct that all contributors are expected to follow. Please read [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) before contributing.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/kube-rde.git
   cd kube-rde
   ```
3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/ORIGINAL_OWNER/kube-rde.git
   ```
4. **Create a branch** for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Setup

### Prerequisites

- Go 1.24 or later
- Docker and Docker Compose
- kubectl
- Access to a Kubernetes cluster (local or cloud)
- Node.js 18+ and npm (for Web UI development)

### Local Development Environment

1. **Build all components**:
   ```bash
   make build
   ```

2. **Deploy to Kubernetes** (for integration testing):
   ```bash
   make deploy
   ```

3. **Develop Web UI**:
   ```bash
   cd web
   npm install
   npm run dev
   ```

See [CLAUDE.md](CLAUDE.md) for detailed development instructions.

## How to Contribute

### Reporting Bugs

Before creating a bug report:
- Check the [issue tracker](https://github.com/OWNER/kube-rde/issues) for existing reports
- Verify the bug exists in the latest version

When creating a bug report, include:
- **Clear title and description**
- **Steps to reproduce** the issue
- **Expected behavior** vs actual behavior
- **Environment details** (OS, Kubernetes version, KubeRDE version)
- **Logs and error messages**
- **Screenshots** if applicable

### Suggesting Enhancements

Enhancement suggestions are welcome! Please:
- Check if the enhancement has already been suggested
- Provide a clear use case for the enhancement
- Explain why this enhancement would be useful to most users
- Consider if it could be implemented as a plugin or extension

### Contributing Code

We welcome code contributions! Here are areas where help is especially appreciated:

- **Bug fixes**: Address issues in the issue tracker
- **Features**: Implement features from the roadmap
- **Documentation**: Improve or add documentation
- **Tests**: Increase test coverage
- **Performance**: Optimize existing code
- **Platform support**: Add support for new cloud platforms

## Pull Request Process

1. **Update your fork** with the latest upstream changes:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Make your changes** following our [coding standards](#coding-standards)

3. **Test your changes**:
   ```bash
   # Run unit tests
   go test ./...

   # Run integration tests
   make test-integration

   # Test Web UI
   cd web && npm test
   ```

4. **Update documentation** if needed:
   - Update README.md for user-facing changes
   - Update CLAUDE.md for developer-facing changes
   - Add inline code comments for complex logic

5. **Commit your changes** with clear commit messages:
   ```bash
   git commit -m "feat: add support for custom resource quotas

   - Implement quota validation in operator
   - Add quota fields to RDEAgent CRD
   - Update documentation with quota examples

   Closes #123"
   ```

   Follow [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat:` for new features
   - `fix:` for bug fixes
   - `docs:` for documentation changes
   - `test:` for test additions/changes
   - `refactor:` for code refactoring
   - `perf:` for performance improvements
   - `chore:` for maintenance tasks

6. **Push to your fork**:
   ```bash
   git push origin feature/your-feature-name
   ```

7. **Create a Pull Request** on GitHub:
   - Use a clear, descriptive title
   - Fill out the PR template completely
   - Link related issues
   - Add screenshots for UI changes
   - Request review from maintainers

8. **Address review feedback**:
   - Make requested changes
   - Push updates to the same branch
   - Respond to comments politely

9. **Merge requirements**:
   - At least one approval from a maintainer
   - All CI checks passing
   - No merge conflicts
   - Documentation updated
   - Tests added/updated as needed

## Coding Standards

### Go Code

- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use `gofmt` for formatting: `make fmt`
- Use `golangci-lint` for linting: `make lint`
- Write clear, self-documenting code with comments for complex logic
- Keep functions focused and small
- Use meaningful variable and function names
- Handle errors explicitly, don't ignore them

**Example**:
```go
// Good
func (s *Server) validateAgentAuth(token string) (*jwt.Token, error) {
    if token == "" {
        return nil, errors.New("token is required")
    }

    parsed, err := s.verifier.Verify(context.Background(), token)
    if err != nil {
        return nil, fmt.Errorf("token verification failed: %w", err)
    }

    return parsed, nil
}

// Bad
func (s *Server) validate(t string) (*jwt.Token, error) {
    p, _ := s.verifier.Verify(context.Background(), t) // Don't ignore errors
    return p, nil
}
```

### TypeScript/React Code (Web UI)

- Follow [React best practices](https://react.dev/learn)
- Use TypeScript for type safety
- Use functional components and hooks
- Format with Prettier: `npm run format`
- Lint with ESLint: `npm run lint`
- Use meaningful component and variable names
- Keep components small and focused
- Write accessible HTML

### YAML/Kubernetes Manifests

- Use 2-space indentation
- Include clear comments for complex configurations
- Follow Kubernetes resource naming conventions
- Add labels and annotations consistently

## Testing Guidelines

### Unit Tests

- Write unit tests for all new functions and components
- Aim for at least 70% code coverage
- Use table-driven tests in Go
- Mock external dependencies

**Example**:
```go
func TestGetRootDomain(t *testing.T) {
    tests := []struct {
        name     string
        hostname string
        want     string
    }{
        {
            name:     "localhost",
            hostname: "localhost",
            want:     "localhost",
        },
        {
            name:     "domain without dot",
            hostname: "example.com",
            want:     ".example.com",
        },
        {
            name:     "domain with dot",
            hostname: ".example.com",
            want:     ".example.com",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := getRootDomain(tt.hostname); got != tt.want {
                t.Errorf("getRootDomain() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Tests

- Test component interactions
- Use a test Kubernetes cluster (kind or minikube)
- Clean up resources after tests

### End-to-End Tests

- Test complete user workflows
- Deploy to a real cluster
- Verify all components work together

## Documentation

### Code Documentation

- Add godoc comments to all exported functions, types, and packages
- Use clear, concise language
- Include examples for complex APIs

**Example**:
```go
// CreateWorkspace creates a new workspace for the specified user.
// It validates the workspace configuration, creates necessary Kubernetes resources,
// and returns the created workspace object.
//
// Example:
//   workspace, err := server.CreateWorkspace(ctx, userID, &WorkspaceConfig{
//       Name: "my-workspace",
//       StorageSize: "10Gi",
//   })
func (s *Server) CreateWorkspace(ctx context.Context, userID string, config *WorkspaceConfig) (*Workspace, error) {
    // Implementation
}
```

### User Documentation

- Update README.md for user-facing changes
- Update docs/ directory for detailed guides
- Include examples and use cases
- Add troubleshooting tips

### Developer Documentation

- Update CLAUDE.md for architecture or build process changes
- Document new environment variables
- Add diagrams for complex flows

## Community

### Getting Help

- **Documentation**: Check [README.md](README.md) and [docs/](docs/)
- **Issues**: Search existing issues or create a new one
- **Discussions**: Use GitHub Discussions for questions and ideas

### Staying Updated

- Watch the repository for updates
- Follow the project roadmap
- Participate in community discussions

### Recognition

Contributors are recognized in:
- Release notes for significant contributions
- CONTRIBUTORS.md file (coming soon)
- GitHub's contributor graph

## Questions?

If you have questions about contributing, please:
1. Check this document and other documentation first
2. Search existing issues and discussions
3. Create a new discussion or issue if needed

Thank you for contributing to KubeRDE!
