# Implementation Plan: Google OAuth Authentication

## Overview

This plan outlines the implementation of **Google OAuth Authentication** to enable secure user authentication and multi-user support for Pennywise.

---

## User Review Required

> [!IMPORTANT]
> **Multi-User Architecture Decision**
> 
> The current implementation appears to be single-user focused (hardcoded email in some places). Moving to production requires:
> - User-scoped data isolation (budgets, transactions, accounts per user)
> - Per-user data access control
> 
> **Question for Review:**
> Should we implement full multi-user data isolation now, or start with basic auth and add isolation incrementally?

> [!WARNING]
> **Security Considerations**
> 
> Authentication requires:
> - Secure JWT token generation and validation
> - Storing user credentials securely
> - HTTPS in production
> - Proper CORS configuration
> 
> Ensure compliance with:
> - Google OAuth policies and security requirements
> - Data protection best practices

---

## Proposed Changes

### Component 1: Frontend Authentication

#### [NEW] [react-frontend/src/features/auth/](file:///Users/ZPM2LPZ/personal/pennywise/react-frontend/src/features/auth/)

Create authentication feature module with Google OAuth integration.

**Files to Create:**
- `components/Login.tsx` - Login page with Google OAuth button
- `components/ProtectedRoute.tsx` - Route wrapper for authenticated pages
- `store/authSlice.ts` - Redux slice for auth state
- `types/auth.types.ts` - Auth-related TypeScript types
- `utils/auth.utils.ts` - Auth helper functions

**Key Functionality:**
- Google OAuth 2.0 sign-in flow (using Google Identity Services)
- Store user info and JWT token in Redux + localStorage
- Automatic token refresh
- Logout functionality
- Protected route wrapper for all app pages

---

#### [MODIFY] [react-frontend/src/app/App.tsx](file:///Users/ZPM2LPZ/personal/pennywise/react-frontend/src/app/App.tsx)

Add authentication routes and protect existing routes.

**Changes:**
- Add `/login` route for Login component
- Wrap existing routes with `ProtectedRoute`
- Add redirect logic (unauthenticated → login, authenticated → dashboard)
- Add logout handler

---

#### [MODIFY] [react-frontend/src/app/store.ts](file:///Users/ZPM2LPZ/personal/pennywise/react-frontend/src/app/store.ts)

Add auth slice to Redux store.

**Changes:**
- Import and add `authSlice` to store reducers
- Configure auth middleware for token management

---

#### [MODIFY] [react-frontend/src/utils/api.ts](file:///Users/ZPM2LPZ/personal/pennywise/react-frontend/src/utils/api.ts)

Add JWT token to API requests.

**Changes:**
- Read auth token from Redux state
- Add `Authorization: Bearer <token>` header to all requests
- Handle 401 responses (redirect to login)
- Implement token refresh logic

---

### Component 2: Backend Authentication

#### [MODIFY] [go-pennywise-api/internal/](file:///Users/ZPM2LPZ/personal/pennywise/backend/go-pennywise-api/internal/)

Implement JWT-based authentication middleware and user management.

**Files to Create:**
- `internal/auth/middleware.go` - JWT validation middleware
- `internal/auth/google_oauth.go` - Google OAuth token verification
- `internal/auth/jwt.go` - JWT generation and validation
- `internal/models/user.go` - User model
- `internal/handlers/auth.go` - Auth endpoints (login, logout, refresh)
- `internal/repository/user_repository.go` - User data access

**Files to Modify:**
- `cmd/api/main.go` - Add auth middleware to routes, add auth endpoints
- All existing handlers - Extract user ID from JWT context (for multi-user isolation)

**Key Functionality:**
- `POST /api/auth/google` endpoint - Exchange Google OAuth token for JWT
- `POST /api/auth/refresh` endpoint - Refresh JWT token
- `POST /api/auth/logout` endpoint - Invalidate refresh token
- JWT middleware - Validate token and inject user context
- User repository - CRUD operations for users in database

**Database Changes:**
- Add `users` table with fields: id, google_id, email, name, picture, created_at, updated_at
- Add `user_id` foreign key to existing tables: budgets, accounts, categories, payees, transactions
- Create migration scripts

---

### Component 3: Database Migrations

#### [NEW] [go-pennywise-api/migrations/](file:///Users/ZPM2LPZ/personal/pennywise/backend/go-pennywise-api/migrations/)

Create database migration scripts for multi-user support.

**Migrations to Create:**

1. **001_create_users_table.sql**
   ```sql
   CREATE TABLE users (
     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
     google_id VARCHAR(255) UNIQUE NOT NULL,
     email VARCHAR(255) UNIQUE NOT NULL,
     name VARCHAR(255),
     picture TEXT,
     created_at TIMESTAMP DEFAULT NOW(),
     updated_at TIMESTAMP DEFAULT NOW()
   );
   ```

2. **002_add_user_id_to_budgets.sql**
   ```sql
   ALTER TABLE budgets ADD COLUMN user_id UUID REFERENCES users(id);
   CREATE INDEX idx_budgets_user_id ON budgets(user_id);
   ```

3. **003_add_user_id_to_accounts.sql** - Similar for accounts table
4. **004_add_user_id_to_transactions.sql** - Similar for transactions table
5. **005_add_user_id_to_categories.sql** - Similar for categories table
6. **006_add_user_id_to_payees.sql** - Similar for payees table

---

## Verification Plan

### Automated Tests

#### Backend Unit Tests

**Authentication Tests** (`go-pennywise-api/internal/auth/middleware_test.go`):
```bash
cd backend/go-pennywise-api
go test ./internal/auth/... -v
```

Tests to write:
- Valid JWT token validation
- Expired JWT token rejection
- Invalid JWT signature rejection
- User context injection
- Google OAuth token verification

#### Frontend Unit Tests

**Auth Slice Tests** (`react-frontend/src/features/auth/store/authSlice.test.ts`):
```bash
cd react-frontend
npm test -- auth
```

Tests to write:
- Login action and state updates
- Logout action and state cleanup
- Token refresh logic
- Protected route access control

### Integration Tests

#### End-to-End Auth Flow

**Manual Test Steps:**
1. Navigate to `http://localhost:3000`
2. Verify redirect to `/login`
3. Click "Sign in with Google"
4. Complete Google OAuth consent
5. Verify redirect to dashboard
6. Verify user info displayed in header
7. Logout and verify redirect to `/login`
8. Login again and verify token persistence

#### Data Isolation (if implementing multi-user)

**Steps:**
1. Create two test users (User A, User B)
2. Login as User A, create budget "Personal"
3. Add account "Chase Checking"
4. Add transaction with payee "Starbucks"
5. Logout, login as User B
6. Verify User B sees NO budgets, accounts, or transactions from User A
7. Create budget "Business" for User B
8. Logout, login as User A
9. Verify User A still sees only "Personal" budget

---

## Dependencies & Integration Points

### External Services
- **Google OAuth 2.0** - User authentication
- **PostgreSQL** - User data storage (existing)

### Configuration Required
- Google OAuth Client ID and Secret (from Google Cloud Console)
- JWT secret key (for signing tokens)
- Database connection strings
- CORS allowed origins

---

## Implementation Order

1. **Phase 1: Database Setup** (Day 1)
   - Create users table migration
   - Add user_id to existing tables
   - **Milestone:** Database ready for multi-user

2. **Phase 2: Backend Auth** (Days 2-3)
   - Implement JWT generation/validation
   - Create Google OAuth verification
   - Build auth endpoints
   - Add JWT middleware
   - **Milestone:** Backend can authenticate users

3. **Phase 3: Frontend Auth** (Days 4-5)
   - Create Login component with Google OAuth
   - Implement auth Redux slice
   - Add ProtectedRoute wrapper
   - Update API client for JWT headers
   - **Milestone:** Users can login with Google

4. **Phase 4: Multi-User Isolation** (Days 6-7)
   - Update all API endpoints to filter by user_id
   - Test data isolation
   - **Milestone:** Each user sees only their own data

---

## Success Criteria

- ✅ Users can sign in with Google OAuth
- ✅ JWT tokens are generated and validated correctly
- ✅ Protected routes redirect unauthenticated users to login
- ✅ User info is displayed in the UI
- ✅ Logout clears session and redirects to login
- ✅ Token refresh works automatically
- ✅ Each user's data is completely isolated (if multi-user implemented)
- ✅ All existing features work with authentication
