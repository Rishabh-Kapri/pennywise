# Product Requirements Document: Pennywise React Frontend

## 1. Executive Summary

**Pennywise** is a personal finance management web application that helps users track their spending, manage budgets, and monitor financial accounts. The React frontend provides an intuitive interface for budget planning, transaction management, and financial reporting following the zero-based budgeting methodology (similar to YNAB - You Need A Budget).

### Product Vision
To provide users with a comprehensive, user-friendly tool for managing personal finances through zero-based budgeting, enabling better financial awareness and control.

---

## 2. Technical Stack

### Core Technologies
- **Framework**: React 19.1.1
- **Language**: TypeScript 5.9.3
- **Build Tool**: Vite 7.1.7
- **State Management**: Redux Toolkit 2.10.1
- **Routing**: React Router DOM 7.9.5

### UI Libraries & Components
- **HeroUI Components**: Autocomplete, Calendar, Popover, Tooltip
- **Icons**: Lucide React 0.553.0
- **Date Handling**: @internationalized/date 3.10.1
- **Number Formatting**: react-number-format 5.4.4
- **Expression Evaluation**: expr-eval 2.0.2
- **Virtualization**: react-window 2.2.3

### Development Tools
- **Linting**: ESLint 9.36.0 with TypeScript support
- **Containerization**: Docker with Nginx
- **Package Manager**: npm

---

## 3. Application Architecture

### Directory Structure
```
src/
├── app/                    # Redux store configuration
│   ├── store.ts           # Central store setup
│   ├── hooks.ts           # Typed Redux hooks
│   └── middlewares.ts     # Custom middleware
├── components/            # Reusable UI components
│   ├── common/           # Shared components
│   ├── layout/           # Layout components (Dashboard, Header, etc.)
│   └── ui/               # Base UI components
├── features/             # Feature-based modules
│   ├── accounts/         # Account management
│   ├── budget/           # Budget planning
│   ├── category/         # Category management
│   ├── payees/           # Payee management
│   ├── reports/          # Financial reports
│   └── transactions/     # Transaction management
├── config/               # Configuration files
├── context/              # React Context providers
├── hooks/                # Custom React hooks
├── styles/               # Global styles
├── types/                # TypeScript type definitions
└── utils/                # Utility functions
```

### State Management Architecture
- **Redux Toolkit** with feature-based slices
- **Custom Middlewares**:
  - `dataFetchMiddleware`: Handles data fetching logic
  - `dateChangeMiddleware`: Manages date-related state changes
  - `budgetUpdateMiddleware`: Coordinates budget updates

### API Integration
- **Custom API Client** (`utils/api.ts`)
- RESTful API communication
- Automatic budget ID header injection
- Centralized error handling
- Base URL configuration via environment variables

---

## 4. Core Features

### 4.1 Budget Management

#### Overview
Users can create and manage multiple budgets, with month-by-month budget planning using zero-based budgeting principles.

#### Key Capabilities
- **Multiple Budget Support**: Create and switch between different budgets
- **Monthly Budget Planning**: Allocate funds to categories for each month
- **Ready to Assign Tracking**: Monitor unallocated funds
- **Budget vs. Actual**: Compare budgeted amounts with actual spending

#### Data Model
```typescript
interface Budget {
  id?: string;
  name: string;
  createdAt?: string;
  updatedAt?: string;
  isSelected?: boolean;
  metadata?: {
    inflowCategoryId: string;
    startingBalPayeeId: string;
    ccGroupId: string;
  };
}
```

#### User Interface
- **Date Selector**: Navigate between months
- **Inflow Amount Display**: Shows "Ready to assign" or "All assigned" status
- **Category Groups**: Collapsible groups with budget allocation inputs
- **Visual Feedback**: Green checkmark when all funds are assigned

---

### 4.2 Category Management

#### Overview
Organize spending into category groups and individual categories for detailed budget tracking.

#### Key Capabilities
- **Category Groups**: Organize related categories together
- **Hierarchical Structure**: Groups contain multiple categories
- **Monthly Tracking**: Track budgeted, activity, and balance per month
- **System Categories**: Special categories for internal operations
- **Collapsible Groups**: Expand/collapse category groups for better organization

#### Data Model
```typescript
interface CategoryGroup {
  id?: string;
  name: string;
  collapsed: boolean;
  balance: Record<string, number>;
  budgeted: Record<string, number>;
  activity: Record<string, number>;
  categories: Category[];
  isSystem: boolean;
}

interface Category {
  id?: string;
  budgetId: string;
  categoryGroupId: string;
  name: string;
  deleted?: boolean;
  hidden?: boolean;
  note?: string | null;
  showBudgetInput?: boolean;
  budgeted: Record<string, number>;
  activity?: Record<string, number>;
  balance?: Record<string, number>;
}
```

#### User Interface
- **Category Group Display**: Shows group totals and categories
- **Inline Budget Input**: Edit budget allocations directly
- **Activity Tracking**: Display spending activity per category
- **Balance Display**: Show remaining balance in each category
- **Amount Cell Component**: Specialized component for currency display and editing

---

### 4.3 Account Management

#### Overview
Manage various financial accounts including checking, savings, credit cards, and tracking accounts.

#### Key Capabilities
- **Multiple Account Types**:
  - Budget Accounts: Checking, Savings, Credit Card
  - Tracking Accounts: Asset, Liability
- **Account Balances**: Real-time balance tracking
- **Transfer Support**: Internal transfers between accounts
- **Account Closure**: Mark accounts as closed without deletion
- **Soft Delete**: Deleted flag for data retention

#### Data Model
```typescript
interface Account {
  id?: string;
  budgetId: string;
  name: string;
  type: BudgetAccountType | TrackingAccountType;
  closed: boolean;
  balance?: number;
  transferPayeeId?: string;
  createdAt?: string;
  updatedAt?: string;
  deleted: boolean;
}

// Account Types
BudgetAccountType: 'checking' | 'savings' | 'creditCard'
TrackingAccountType: 'asset' | 'liability'
```

#### User Interface
- **Account List**: Display all accounts with balances
- **Account Type Indicators**: Visual distinction between account types
- **Balance Display**: Current balance for each account

---

### 4.4 Transaction Management

#### Overview
Record and manage all financial transactions with detailed categorization and search capabilities.

#### Key Capabilities
- **Transaction Entry**: Add income and expenses
- **Expression Evaluation**: Calculate amounts using mathematical expressions (e.g., "50+25")
- **Account Filtering**: View transactions for specific accounts or all accounts
- **Transfer Tracking**: Link transfer transactions between accounts
- **Transaction Sources**: Track whether transactions are manual or imported (MLP)
- **Virtualized List**: Efficient rendering of large transaction lists
- **Inline Editing**: Edit transactions directly in the list
- **Search Functionality**: Search transactions by various criteria

#### Data Model
```typescript
interface Transaction {
  id?: string;
  budgetId: string;
  date: string;
  amount?: number;
  outflow: number | null;
  inflow: number | null;
  balance: number;
  note?: string;
  source: 'PENNYWISE' | 'MLP';
  transferTransactionId: string | null;
  transferAccountId: string | null;
  accountName: string;
  accountId: string;
  payeeName: string;
  payeeId: string;
  categoryName: string | null;
  categoryId: string | null;
}
```

#### User Interface
- **Transaction Header**: Account name, balance, and action buttons
- **Add Expense Button**: Quick transaction entry
- **Search Bar**: Filter transactions
- **Column Layout**:
  - All Accounts View: Date, Account, Payee, Category, Note, Outflow, Inflow, Balance
  - Specific Account View: Date, Payee, Category, Note, Outflow, Inflow, Balance
- **Virtualized List**: Performance-optimized scrolling
- **Inline Editing**: Click to edit any transaction field
- **Expression Parser**: Automatically evaluates math expressions in amount fields

---

### 4.5 Payee Management

#### Overview
Track who money is paid to or received from, with special handling for transfer payees.

#### Key Capabilities
- **Payee Tracking**: Maintain a list of payees
- **Transfer Payees**: Special payees representing account transfers
- **Soft Delete**: Maintain payee history
- **Auto-complete**: Quick payee selection in transactions

#### Data Model
```typescript
interface Payee {
  id?: string;
  budgetId: string;
  name: string;
  transferAccountId: string | null;
  createdAt?: string;
  updatedAt?: string;
  deleted: boolean;
}
```

---

### 4.6 Reports (Planned)

#### Overview
Financial reporting and visualization features (currently in development).

#### Planned Reports
- **Spending Report**: Analyze spending patterns by category
- **Income vs. Expense Report**: Compare income and expenses over time
- **Net Worth Report**: Track assets and liabilities

#### Current Status
- Report components exist but are not yet implemented
- Empty placeholder files for future development

---

### 4.7 Dashboard

#### Overview
Central hub for quick overview of financial status (currently minimal implementation).

#### Current Features
- **User Greeting**: Personalized welcome message
- **State Monitoring**: Development-mode state logging

#### Potential Enhancements
- Budget summary widgets
- Recent transactions
- Spending alerts
- Financial goals tracking
- Quick action buttons

---

## 5. User Interface Components

### Layout Components
- **Layout**: Main application shell with header and content area
- **Dashboard**: Landing page and overview
- **Header Context**: Dynamic header content based on current route

### Common Components
- **DateSelector**: Month/year navigation
- **Skeleton**: Loading state placeholders
- **Dropdown Components**: Account, Category, and Payee selection

### Feature-Specific Components
- **CategoryGroup**: Display and manage category groups
- **CategoryItemList**: List of categories within a group
- **AmountCell**: Specialized currency input/display
- **TransactionRow**: Individual transaction in the list
- **TransactionCell**: Editable transaction fields
- **Activity**: Budget activity display

---

## 6. Key User Flows

### 6.1 Budget Planning Flow
1. User selects a budget from available budgets
2. User navigates to Budget page
3. User selects target month using DateSelector
4. System displays "Ready to assign" amount
5. User allocates funds to categories
6. System updates "Ready to assign" amount in real-time
7. When all funds allocated, system shows "All assigned" with checkmark

### 6.2 Transaction Entry Flow
1. User navigates to Transactions page (all or specific account)
2. User clicks "Add Expense" button
3. System creates new transaction row at top of list
4. User enters transaction details:
   - Date (defaults to today)
   - Payee (with autocomplete)
   - Category (with autocomplete)
   - Amount (supports expressions like "50+25")
   - Note (optional)
5. User tabs through fields or clicks elsewhere
6. System evaluates expressions and saves transaction
7. System updates account balance and category activity

### 6.3 Account Management Flow
1. User views account list in sidebar
2. User clicks on specific account
3. System navigates to account-specific transaction view
4. User sees account name, balance, and transactions
5. User can add transactions or view/edit existing ones

---

## 7. Technical Implementation Details

### State Management Patterns
- **Feature-based Slices**: Each feature has its own Redux slice
- **Async Thunks**: API calls handled via Redux Toolkit thunks
- **Selectors**: Memoized selectors for derived state
- **Loading States**: Consistent loading state enum (`IDLE`, `PENDING`, `SUCCESS`, `FAILED`)

### Performance Optimizations
- **Code Splitting**: Lazy loading of route components
- **Virtualization**: react-window for large transaction lists
- **Dynamic Row Heights**: Adaptive row sizing for better UX
- **Memoization**: useMemo and useCallback for expensive computations

### API Communication
- **Centralized Client**: Single ApiClient class
- **Automatic Headers**: Budget ID automatically included in requests
- **Type Safety**: Generic types for request/response
- **Error Handling**: Centralized error parsing and logging

### Custom Hooks
- **useDebounce**: Debounce input changes
- **useDropdown**: Manage dropdown state for autocomplete
- **useDynamicRowHeight**: Calculate dynamic row heights for virtualized lists
- **useAppSelector/useAppDispatch**: Typed Redux hooks

---

## 8. Environment Configuration

### Environment Variables
```
VITE_API_URL - Backend API base URL (default: http://localhost:5151/api)
```

### Build Configurations
- **Development**: Hot module replacement, source maps
- **Production**: Optimized bundle, minification
- **Docker**: Nginx-based deployment

---

## 9. Current Limitations & Known Issues

### Implemented Features with Gaps
1. **Dashboard**: Minimal implementation, needs enhancement
2. **Reports**: Placeholder components, not functional
3. **Search**: UI exists but functionality not fully implemented
4. **Mobile Responsiveness**: Desktop-focused, mobile views exist but may need refinement

### Missing Features
1. **User Authentication**: No login/signup flow
2. **Data Import**: No CSV/OFX import functionality
3. **Goals**: Category goals not implemented
4. **Reconciliation**: Account reconciliation feature missing
5. **Multi-currency**: Single currency support only
6. **Recurring Transactions**: No automated recurring transaction support
7. **Budget Templates**: No template system for new budgets

---

## 10. Potential Feature Enhancements

### High Priority
1. **Enhanced Dashboard**
   - Budget overview widgets
   - Spending trends visualization
   - Quick stats (total income, expenses, net worth)
   - Recent transactions list
   - Budget health indicators

2. **Reports Implementation**
   - Spending by category charts
   - Income vs. expense trends
   - Net worth tracking over time
   - Custom date range selection
   - Export to PDF/CSV

3. **Search & Filtering**
   - Full-text transaction search
   - Advanced filters (date range, amount range, categories)
   - Saved search queries
   - Transaction tagging

4. **Mobile Optimization**
   - Responsive design improvements
   - Touch-friendly interfaces
   - Mobile-specific navigation
   - Swipe gestures for actions

### Medium Priority
5. **Category Goals**
   - Target balance goals
   - Monthly funding goals
   - Progress tracking
   - Goal templates

6. **Account Reconciliation**
   - Mark transactions as cleared
   - Reconciliation workflow
   - Balance verification
   - Reconciliation history

7. **Data Import/Export**
   - CSV import
   - OFX/QFX file support
   - Bank integration (Plaid)
   - Export to various formats

8. **Recurring Transactions**
   - Schedule recurring income/expenses
   - Auto-create transactions
   - Recurring transaction templates
   - Skip/modify upcoming instances

### Low Priority
9. **Budget Templates**
   - Save budget as template
   - Apply template to new months
   - Share templates (future multi-user)

10. **Multi-currency Support**
    - Multiple currency accounts
    - Exchange rate tracking
    - Currency conversion

11. **Notifications & Alerts**
    - Budget overspending alerts
    - Bill reminders
    - Low balance warnings
    - Goal achievement notifications

12. **Collaboration Features**
    - Shared budgets
    - User permissions
    - Activity log
    - Comments on transactions

---

## 11. Data Flow Architecture

### Application Initialization
1. App loads → Redux store initialized
2. Middleware configured (data fetch, date change, budget update)
3. API client configured with store access
4. Router initialized with lazy-loaded routes

### Typical Data Flow
1. **User Action** → Component dispatches Redux action
2. **Middleware** → Intercepts action, may trigger side effects
3. **Thunk** → Makes API call via ApiClient
4. **API Client** → Adds headers (budget ID), sends request
5. **Response** → Parsed and returned to thunk
6. **Reducer** → Updates state based on action
7. **Selector** → Derives data from state
8. **Component** → Re-renders with new data

### Budget Context Flow
- Budget ID stored in Redux state
- API client reads budget ID from state
- Automatically includes in request headers
- Backend filters data by budget ID

---

## 12. Testing Strategy (Recommended)

### Current State
- No test files identified in the codebase
- Testing infrastructure needs to be established

### Recommended Testing Approach

#### Unit Tests
- Redux slices and reducers
- Utility functions
- Custom hooks
- Selectors

#### Integration Tests
- API client functionality
- Redux middleware
- Component integration with Redux

#### E2E Tests
- Critical user flows (budget planning, transaction entry)
- Multi-page workflows
- Data persistence

#### Suggested Tools
- **Unit/Integration**: Vitest (Vite-native)
- **Component Testing**: React Testing Library
- **E2E**: Playwright or Cypress

---

## 13. Deployment Architecture

### Docker Configuration
- **Base Image**: Node.js for build
- **Production Server**: Nginx
- **Static Assets**: Served via Nginx
- **Configuration**: nginx.conf for routing

### Build Process
1. TypeScript compilation
2. Vite build (bundling, minification)
3. Docker image creation
4. Nginx configuration
5. Container deployment

---

## 14. Accessibility Considerations

### Current State
- Basic semantic HTML structure
- Keyboard navigation support (Escape key handling)
- Focus management in modals/dropdowns

### Recommended Enhancements
- ARIA labels and roles
- Screen reader optimization
- Keyboard shortcuts documentation
- High contrast mode support
- Focus indicators
- Skip navigation links

---

## 15. Security Considerations

### Current Implementation
- Environment-based configuration
- HTTPS support (via deployment)
- No sensitive data in client-side code

### Recommended Enhancements
- Authentication/Authorization implementation
- CSRF protection
- XSS prevention (React provides basic protection)
- Content Security Policy
- Secure session management
- API rate limiting
- Input validation and sanitization

---

## 16. Success Metrics (Proposed)

### User Engagement
- Daily/Monthly Active Users
- Average session duration
- Feature adoption rates
- Transaction entry frequency

### Financial Health Indicators
- Percentage of users with fully allocated budgets
- Average time to budget reconciliation
- Number of categories used per user
- Transaction categorization rate

### Technical Performance
- Page load time
- Time to interactive
- API response times
- Error rates
- Bundle size

---

## 17. Roadmap Considerations

### Phase 1: Foundation (Current State)
- ✅ Core budget management
- ✅ Transaction tracking
- ✅ Account management
- ✅ Category system
- ⚠️ Basic dashboard (needs enhancement)

### Phase 2: Enhancement
- 🔲 Implement reports
- 🔲 Enhanced dashboard
- 🔲 Search and filtering
- 🔲 Mobile optimization
- 🔲 Testing infrastructure

### Phase 3: Advanced Features
- 🔲 Goals and targets
- 🔲 Reconciliation
- 🔲 Data import/export
- 🔲 Recurring transactions
- 🔲 Multi-currency

### Phase 4: Scale & Collaborate
- 🔲 User authentication
- 🔲 Shared budgets
- 🔲 Bank integration
- 🔲 Notifications
- 🔲 Mobile app

---

## 18. Conclusion

The Pennywise React frontend is a well-architected personal finance management application with a solid foundation in core budgeting features. The application follows modern React best practices with TypeScript, Redux Toolkit for state management, and a clean feature-based architecture.

### Strengths
- Clean, modular architecture
- Type-safe implementation with TypeScript
- Efficient state management with Redux Toolkit
- Performance optimizations (lazy loading, virtualization)
- Solid core feature set for zero-based budgeting

### Areas for Growth
- Dashboard needs significant enhancement
- Reports feature requires implementation
- Testing infrastructure needs to be established
- Mobile experience could be improved
- Advanced features (goals, reconciliation, recurring transactions) are missing

The application is well-positioned for future enhancements and has a clear path forward for becoming a comprehensive personal finance management solution.
