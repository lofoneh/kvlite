# Contributing to kvlite

Thank you for your interest in contributing to kvlite! This document provides guidelines and instructions for contributing.

## Ways to Contribute

- **Bug Reports**: Found a bug? Open an issue with details
- **Feature Requests**: Have an idea? We'd love to hear it
- **Code Contributions**: Bug fixes, new features, improvements
- **Documentation**: Improve docs, add examples, fix typos
- **Testing**: Add tests, improve coverage, report edge cases

## Getting Started

### Prerequisites

- Go 1.21 or later
- Make (optional, for build commands)
- Git

### Setup

1. Fork the repository on GitHub

2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/kvlite.git
   cd kvlite
   ```

3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/lofoneh/kvlite.git
   ```

4. Install dependencies:
   ```bash
   make deps
   ```

5. Run tests to verify setup:
   ```bash
   make test
   ```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Adding or improving tests
- `chore/` - Maintenance tasks

### 2. Make Your Changes

- Write clean, readable code
- Follow existing code style
- Add tests for new functionality
- Update documentation as needed

### 3. Test Your Changes

```bash
# Run all tests
make test

# Run tests with race detector
make test-race

# Run tests with coverage
make test-cover

# Run linter
make lint

# Format code
make fmt

# Run full check
make check
```

### 4. Commit Your Changes

Write clear, concise commit messages:

```bash
# Good examples
git commit -m "Add TTL support for MSET command"
git commit -m "Fix race condition in connection pool"
git commit -m "Update README with Docker instructions"

# Bad examples
git commit -m "fix stuff"
git commit -m "updates"
```

### 5. Push and Create PR

```bash
git push origin feature/your-feature-name
```

Then open a Pull Request on GitHub.

## Code Style

### Go Code

- Follow standard Go conventions
- Run `gofmt` before committing
- Use meaningful variable names
- Add comments for exported functions
- Keep functions focused and small

```go
// Good
func (s *Store) Get(key string) (string, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    entry, exists := s.data[key]
    if !exists {
        return "", false
    }
    return entry.Value, true
}

// Avoid
func (s *Store) g(k string) (string, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    e, ex := s.data[k]
    if !ex { return "", false }
    return e.Value, true
}
```

### Tests

- Write tests for all new functionality
- Use table-driven tests where appropriate
- Test edge cases and error conditions

```go
func TestStore_Get(t *testing.T) {
    tests := []struct {
        name     string
        key      string
        setup    func(*Store)
        want     string
        wantOk   bool
    }{
        {
            name:   "existing key",
            key:    "foo",
            setup:  func(s *Store) { s.Set("foo", "bar", 0) },
            want:   "bar",
            wantOk: true,
        },
        {
            name:   "missing key",
            key:    "missing",
            setup:  func(s *Store) {},
            want:   "",
            wantOk: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            s := NewStore()
            tt.setup(s)
            got, ok := s.Get(tt.key)
            if got != tt.want || ok != tt.wantOk {
                t.Errorf("Get() = %v, %v; want %v, %v", got, ok, tt.want, tt.wantOk)
            }
        })
    }
}
```

## Pull Request Guidelines

### Before Submitting

- [ ] Tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated (if needed)
- [ ] Commit messages are clear

### PR Description

Include:
- What changes were made
- Why the changes were made
- How to test the changes
- Any breaking changes

### Review Process

1. A maintainer will review your PR
2. Address any feedback
3. Once approved, your PR will be merged

## Reporting Issues

### Bug Reports

Include:
- kvlite version
- Go version
- Operating system
- Steps to reproduce
- Expected vs actual behavior
- Error messages or logs

### Feature Requests

Include:
- Use case description
- Proposed solution
- Alternative solutions considered

## Community

- Be respectful and inclusive
- Help others when you can
- Follow the [Code of Conduct](CODE_OF_CONDUCT.md)

## Questions?

- Open an issue for questions
- Tag it with the `question` label

Thank you for contributing!
