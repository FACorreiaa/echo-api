---
description: How to develop and run the Go API server
---

# Go API Development Workflow

## Start Development Server

### Run API Server
// turbo
1. Start the API server:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && go run ./cmd/api
```

### With Hot Reload (using air)
2. Start with hot reload:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && air
```

## Build Commands

### Build Binary
// turbo
3. Build the API binary:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && go build -o bin/api ./cmd/api
```

### Build for Production
4. Build optimized binary:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/api ./cmd/api
```

## Testing

### Run All Tests
// turbo
5. Run all tests:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && go test ./...
```

### Run Tests with Coverage
6. Run tests with coverage report:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && go test -cover -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

### Run Specific Package Tests
7. Run tests for a specific package:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && go test -v ./internal/domain/<package>/...
```

## Code Quality

### Format Code
// turbo
8. Format all Go files:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && go fmt ./...
```

### Run Linter
// turbo
9. Run golangci-lint:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && golangci-lint run
```

### Vet Code
// turbo
10. Run go vet:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && go vet ./...
```

## Dependencies

### Download Dependencies
// turbo
11. Download all dependencies:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && go mod download
```

### Tidy Dependencies
// turbo
12. Clean up go.mod and go.sum:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && go mod tidy
```

### Update Dependencies
13. Update all dependencies:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && go get -u ./... && go mod tidy
```

## Database

### Run Migrations
14. Run database migrations (if using golang-migrate):
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && migrate -path ./migrations -database "$DATABASE_URL" up
```

### Create New Migration
15. Create a new migration:
```bash
cd /Users/fernando_idwell/Projects/FinanceTrackerEcho/EchoAPI && migrate create -ext sql -dir migrations -seq <migration_name>
```

## Project Structure

```
EchoAPI/
├── cmd/api/           # Application entry point
│   ├── main.go
│   └── dependencies.go
├── internal/
│   ├── domain/        # Domain-driven design modules
│   │   ├── auth/      # Authentication domain
│   │   ├── balance/   # Balance engine
│   │   ├── finance/   # Transaction management
│   │   ├── goal/      # Goals tracking
│   │   └── plan/      # Budgeting plans
│   └── pkg/           # Shared internal packages
├── migrations/        # Database migrations
└── docs/              # API documentation
```

## Creating a New Domain

1. Create domain folder: `internal/domain/<name>/`
2. Add files:
   - `handler.go` - Connect-RPC handlers
   - `service.go` - Business logic
   - `repository.go` - Data access
   - `models.go` - Domain types (if needed)
3. Wire up in `cmd/api/dependencies.go`
4. Add routes in router
