# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Overview

Pennywise is a personal finance application with a microservices architecture consisting of:
- **Frontend**: Angular 17 SPA with Firebase integration and NGXS state management
- **Go API**: RESTful API server with clean architecture (handler/service/repository pattern)
- **Go Gmail Service**: Email processing service for transaction extraction
- **Python MLP**: Machine learning service for transaction categorization using sentence transformers

## Development Commands

### Frontend (Angular)
- **Start dev server**: `cd frontend && npm start` (runs on port 5000)
- **Build**: `npm run build` (production) or `npm run watch` (development)
- **Test**: `npm run test` (all tests)
- **Single test**: `ng test --include="**/component-name.component.spec.ts"`

### Go API
- **Build**: `cd backend/go-pennywise-api && go build ./cmd/api`
- **Test**: `go test ./...` (all tests) or `go test -run TestFunctionName ./internal/service` (single test)
- **Lint**: `go fmt ./...` and `go vet ./...`
- **Run**: Built binary runs on port 5151

### Go Gmail Service
- **Build**: `cd backend/go-gmail && go build`
- **Test**: `go test ./...`
- **Lint**: `go fmt ./...` and `go vet ./...`

### Python MLP Service
- **Setup**: `cd backend/python-mlp && pip install -r requirements.txt`
- **Run**: `python mlp_predict_server.py` (runs on port 8000)

### Docker (Full Stack)
- **Start all services**: `cd backend && docker-compose up`
- **Ports**: Frontend (5000), Gmail (5000), MLP (5050), API (5151)

## Architecture

### Frontend Architecture
- **State Management**: NGXS store with Firebase Firestore integration via `@ngxs-labs/firestore-plugin`
- **UI Framework**: Angular 17 with Tailwind CSS, Flowbite components, and Heroicons
- **Charts**: Highcharts for financial visualizations
- **Key Services**:
  - `DatabaseService`: Firebase Firestore operations
  - `HttpService`: API communication with custom headers interceptor
  - `HelperService`: Utility functions
- **Models**: TypeScript interfaces for Account, Transaction, Budget, Category, etc.

### Backend Architecture (Go API)
- **Pattern**: Clean architecture with dependency injection
- **Layers**:
  - `cmd/api`: Application entry point and routing setup
  - `internal/handler`: HTTP request handlers (Gin framework)
  - `internal/service`: Business logic layer
  - `internal/repository`: Database access layer (PostgreSQL)
  - `internal/model`: Domain models
  - `pkg/`: Shared utilities
- **Database**: PostgreSQL with pgx driver
- **API**: RESTful endpoints with CORS enabled for localhost:5000

### Service Communication
- **Go API ↔ Frontend**: HTTP/JSON via custom headers interceptor (X-Budget-ID)
- **Gmail Service ↔ Python MLP**: HTTP requests for transaction categorization
- **Python MLP**: Sentence transformer models for semantic similarity matching

### Key Integrations
- **Firebase**: Frontend authentication and Firestore for real-time data
- **Google Gmail API**: Automated email parsing for transaction extraction
- **Machine Learning**: Pre-trained models for payee, category, and account prediction

## Code Style Guidelines

### Frontend (Angular/TypeScript)
- **Component Architecture**: Mix of standalone and module-based (follow existing patterns)
- **Dependency Injection**: Constructor injection preferred, avoid `inject()` for consistency
- **Import Order**: Angular core, third-party libraries, then local imports (models, services, constants)
- **Naming**: PascalCase for classes/interfaces, camelCase for methods/properties, kebab-case for files
- **Styling**: SCSS with Tailwind CSS classes, component-scoped styles

### Backend (Go)
- **Project Structure**: `internal/` for private code, `pkg/` for shared utilities
- **Import Grouping**: Standard library, third-party (gin, uuid, pgx), then local packages
- **Error Handling**: Return error as last value, appropriate HTTP status codes
- **JSON Responses**: Use `gin.H{"error": "message"}` format for errors
- **Dependency Injection**: Constructor pattern with interfaces in service layers

### Python
- **ML Models**: Use sentence-transformers for embeddings and similarity matching
- **API**: FastAPI-style endpoints for model inference
- **File Structure**: Separate model files (.parms) for different prediction types

## Environment Setup

### Required Environment Files
- `backend/go-gmail/.env`: Gmail API credentials and configuration
- `backend/go-pennywise-api/.env`: Database connection and API configuration
- Service account JSON for Google Cloud Platform authentication

### Database
- PostgreSQL database required for Go API
- Firestore for frontend real-time features
- Connection managed through `internal/db/db.go`

## Testing Strategy

### Frontend
- Karma + Jasmine for unit tests
- Component testing with Angular testing utilities
- Test files co-located with components (.spec.ts)

### Backend
- Go standard testing package
- Repository layer testing with database mocks
- Service layer business logic testing
- HTTP handler testing with Gin test context
