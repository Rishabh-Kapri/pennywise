# Agent Guidelines for Pennywise

## Build/Lint/Test Commands
- **Frontend**: 
  - Build: `cd frontend && npm run build` (production), `npm run watch` (development)
  - Test: `npm run test` (all tests), `ng test --include="**/component-name.component.spec.ts"` (single test)
  - Lint: No explicit lint command - follow TypeScript strict mode rules
- **Go Gmail Service**: 
  - Build: `cd backend/go-gmail && go build`
  - Test: `go test ./...` (all tests), `go test -run TestFunctionName ./pkg/parser` (single test)
  - Lint: `go fmt ./...` and `go vet ./...`
- **Go API**: 
  - Build: `cd backend/go-pennywise-api && go build ./cmd/api` 
  - Test: `go test ./...` (all tests), `go test -run TestFunctionName ./internal/service` (single test)
  - Lint: `go fmt ./...` and `go vet ./...`
- **Python MLP**: `cd backend/python-mlp && pip install -r requirements.txt && python mlp_predict_server.py`

## Code Style Guidelines

### Frontend (Angular 17 + TypeScript)
- Components: Mix of standalone and module-based components (follow existing patterns)
- Imports: Angular core first, third-party libraries, then local imports (models, services, constants)
- DI: Constructor injection preferred (existing codebase pattern), avoid `inject()` for consistency
- Naming: PascalCase for classes/interfaces, camelCase for methods/properties, kebab-case for files
- Types: Explicit interfaces for models, strict TypeScript mode enabled
- Styling: SCSS files, Tailwind CSS classes, component-scoped styles

### Backend (Go)
- Structure: `internal/` for private API code, `pkg/` for shared utilities
- Imports: Standard library, third-party (gin, uuid), then local packages grouped
- Naming: PascalCase for exported, camelCase for private (Go conventions)
- Error handling: Return error as last value, handle all errors with appropriate HTTP status
- HTTP: Gin framework, JSON responses with `gin.H{"error": "message"}` format
- Interfaces: Define in handler/service layers, implement dependency injection pattern