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

// Websocket error codes
const (
	CodeWebsocketClientError Code = "WS_CLIENT_ERROR"
	CodeWebsocketMessageRead Code = "WS_MESSAGE_READ_ERROR"
)

// User error codes
const (
	CodeUserLookupFailed Code = "USER_LOOKUP_FAILED"
	CodeUserUpdateFailed Code = "USER_UPDATE_FAILED"
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
	CodePredictionCreateFailed  Code = "PREDICTION_CREATE_FAILED"
	CodePredictionUpdateFailed  Code = "PREDICTION_UPDATE_FAILED"
	CodePredictionDeleteFailed  Code = "PREDICTION_DELETE_FAILED"
)

// Payee/Account/Category error codes
const (
	CodePayeeLookupFailed    Code = "PAYEE_LOOKUP_FAILED"
	CodePayeeCreateFailed    Code = "PAYEE_CREATE_FAILED"
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

// Agent
const (
	CodeLLMNotConfigured              Code = "AGENT_LLM_NOT_CONFIGURED"
	CodeAgentCreateFailed             Code = "AGENT_CREATE_FAILED"
	CodeAgentRunNotFound              Code = "AGENT_RUN_NOT_FOUND"
	CodeAgentRunLookupFailed          Code = "AGENT_RUN_LOOKUP_FAILED"
	CodeAgentRunCreateFailed          Code = "AGENT_RUN_CREATE_FAILED"
	CodeAgentRunUpdateFailed          Code = "AGENT_RUN_UPDATE_FAILED"
	CodeAgentConversationNotFound     Code = "AGENT_CONVERSATION_NOT_FOUND"
	CodeAgentConversationLookupFailed Code = "AGENT_CONVERSATION_LOOKUP_FAILED"
	CodeAgentConversationCreateFailed Code = "AGENT_CONVERSATION_CREATE_FAILED"
	CodeAgentMessageLookupFailed      Code = "AGENT_MESSAGE_LOOKUP_FAILED"
	CodeAgentMessageCreateFailed      Code = "AGENT_MESSAGE_CREATE_FAILED"
	CodeAgentDispatchFailed           Code = "AGENT_DISPATCH_FAILED"
	CodeAgentCancelFailed             Code = "AGENT_CANCEL_FAILED"
)

// Tool
const (
	CodeToolNotFound    Code = "TOOL_NOT_FOUND"
	CodeToolExecuteFail Code = "TOOL_EXECUTION_FAILED"
)
