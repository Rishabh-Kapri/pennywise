package errs

type Code string

// Generic error codes
const (
	CodeInternalError   Code = "INTERNAL_ERROR"
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
)

// Auth error codes
const (
	CodeAuthLookupFailed Code = "AUTH_LOOKUP_FAILED"
	CodeAuthCreateFailed Code = "AUTH_CREATE_FAILED"
)

// HTTP client error codes
const (
	CodeHTTPClientError Code = "HTTP_CLIENT_ERROR"
)

// Transaction/Transfer/Prediction error codes
const (
	CodeTransactionCreateFailed Code = "TRANSACTION_CREATE_FAILED"
	CodeTransactionNotCreated   Code = "TRANSACTION_NOT_CREATED"
	CodeTransactionUpdateFailed Code = "TRANSACTION_UPDATE_FAILED"
	CodeTransactionLookupFailed Code = "TRANSACTION_LOOKUP_FAILED"
	CodeTransactionDeleteFailed Code = "TRANSACTION_DELETE_FAILED"
	CodeTransferCreateFailed    Code = "TRANSFER_CREATE_FAILED"
	CodeTransferNotCreated      Code = "TRANSFER_NOT_CREATED"
	CodeTransferLinkFailed      Code = "TRANSFER_LINK_FAILED"
	CodeBudgetLookupFailed      Code = "BUDGET_LOOKUP_FAILED"
	CodePredictionLookupFailed  Code = "PREDICTION_LOOKUP_FAILED"
	CodePredictionUpdateFailed  Code = "PREDICTION_UPDATE_FAILED"
	CodePredictionDeleteFailed  Code = "PREDICTION_DELETE_FAILED"
)

// Payee/Account/Category error codes
const (
	CodePayeeLookupFailed    Code = "PAYEE_LOOKUP_FAILED"
	CodeAccountLookupFailed  Code = "ACCOUNT_LOOKUP_FAILED"
	CodeAccountCreateFailed  Code = "ACCOUNT_CREATE_FAILED"
	CodeCategoryLookupFailed Code = "CATEGORY_LOOKUP_FAILED"
)

// Monthly budget error codes
const (
	CodeMonthlyBudgetLookupFailed Code = "MONTHLY_BUDGET_LOOKUP_FAILED"
	CodeMonthlyBudgetCreateFailed Code = "MONTHLY_BUDGET_CREATE_FAILED"
	CodeMonthlyBudgetUpdateFailed Code = "MONTHLY_BUDGET_UPDATE_FAILED"
)
