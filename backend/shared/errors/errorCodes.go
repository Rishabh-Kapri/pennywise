package errs

type Code string

// Generic error codes
const (
	CodeInternalError   Code = "INTERNAL_ERROR"
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	// CodeNotImplemented: the code path exists but its logic isn't written yet.
	CodeNotImplemented Code = "NOT_IMPLEMENTED"
)

// Auth error codes
const (
	CodeAuthLookupFailed Code = "AUTH_LOOKUP_FAILED"
	CodeAuthCreateFailed Code = "AUTH_CREATE_FAILED"
	// CodeAuthInvalidCredentials: a credential was presented and rejected
	// (malformed, unknown, inactive, expired or revoked). Maps to 401.
	CodeAuthInvalidCredentials Code = "AUTH_INVALID_CREDENTIALS"
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
	// CodeTransactionLearnFailed: a transaction.learn job couldn't save what it learned.
	CodeTransactionLearnFailed  Code = "TRANSACTION_LEARN_FAILED"
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

	// CodeTransactionInvalidStatus: the transaction's status can't move to the
	// one asked for (model.TransactionStatus.CanMoveTo), e.g. approving a
	// manual transaction. Maps to 409.
	CodeTransactionInvalidStatus Code = "TRANSACTION_INVALID_STATUS"
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
	CodeLLMNotConfigured Code = "AGENT_LLM_NOT_CONFIGURED"
	// CodeAIProviderNotConfigured: the budget owner has no enabled AI provider
	// (user_llm_providers). Not retryable until they add one.
	CodeAIProviderNotConfigured       Code = "AI_PROVIDER_NOT_CONFIGURED"
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

// System pipelines (shared/pipeline)
const (
	// CodePipelineInvalid: a pipeline file or the pipeline schema is broken.
	// Found at startup; the service can't run until the file is fixed.
	CodePipelineInvalid Code = "PIPELINE_INVALID"
	// CodePipelineInvalidOverride: a budget's change to a pipeline step was
	// rejected. The message is written for the user. Maps to 400.
	CodePipelineInvalidOverride Code = "PIPELINE_INVALID_OVERRIDE"
)

// Prediction adapters (cipher)
const (
	// CodeAdapterNotConfigured: config names an adapter the factory doesn't know.
	CodeAdapterNotConfigured Code = "ADAPTER_NOT_CONFIGURED"
	// CodeDecisionsNotConfigured: the decisions adapter is selected but its
	// URL, key or model is missing.
	CodeDecisionsNotConfigured Code = "DECISIONS_NOT_CONFIGURED"
	// CodeDecisionsRequestFailed: the decisions API call failed; StatusOf
	// gives the upstream HTTP status when there was one.
	CodeDecisionsRequestFailed Code = "DECISIONS_REQUEST_FAILED"
	// CodeDecisionsAnswerMissing: the API answered without a usable answer
	// for a question that was asked.
	CodeDecisionsAnswerMissing Code = "DECISIONS_ANSWER_MISSING"
)

// Ingest
const (
	// CodeIngestInvalidMessage: the message is missing something it needs
	// (body, sender, a budget, a dedupe ID). Maps to 400.
	CodeIngestInvalidMessage Code = "INGEST_INVALID_MESSAGE"
	// CodeIngestQueueUnavailable: no job queue is configured (JOB_DRIVER
	// unset), so messages can't be accepted. Maps to 503.
	CodeIngestQueueUnavailable Code = "INGEST_QUEUE_UNAVAILABLE"
)

// Mail connections (ingest)
const (
	CodeConnectionNotFound     Code = "CONNECTION_NOT_FOUND"
	CodeConnectionLookupFailed Code = "CONNECTION_LOOKUP_FAILED"
	CodeConnectionUpdateFailed Code = "CONNECTION_UPDATE_FAILED"
)

// Mail providers (ingest)
const (
	// CodeMailNotConfigured: the provider's app settings (client ID, Pub/Sub
	// topic, ...) are missing.
	CodeMailNotConfigured Code = "MAIL_NOT_CONFIGURED"
	// CodeMailInvalidConnection: the connection row can't be used as is
	// (wrong credentials type, missing or malformed cursor).
	CodeMailInvalidConnection Code = "MAIL_INVALID_CONNECTION"
	// CodeMailAuthFailed: the provider refused the stored credentials
	// (revoked or expired grant). Retrying won't help; the user must reconnect.
	CodeMailAuthFailed Code = "MAIL_AUTH_FAILED"
	// CodeMailNoRefreshToken: the provider granted access but gave no long-lived
	// token, usually because it didn't show the consent screen.
	CodeMailNoRefreshToken Code = "MAIL_NO_REFRESH_TOKEN"
	// CodeMailCursorExpired: the provider no longer has history that far
	// back. Call Watch for a fresh cursor (and Search to fill the gap).
	CodeMailCursorExpired Code = "MAIL_CURSOR_EXPIRED"
	// CodeMailRequestFailed: a provider API call failed; StatusOf gives the
	// HTTP status when there was one.
	CodeMailRequestFailed Code = "MAIL_REQUEST_FAILED"
)

// Jobs
const (
	CodeJobDatabaseURLMissing Code = "JOB_DATABASE_URL_MISSING"
	CodeJobStoreURLMissing    Code = "JOB_DATABASE_STORE_URL_MISSING"
	CodeJobInvalidOption      Code = "JOB_INVALID_OPTION"
	// CodeJobInvalidPayload: a job's payload can't be decoded, or it has no
	// budget. A retry can't fix it; it dies after MaxAttempts.
	CodeJobInvalidPayload    Code = "JOB_INVALID_PAYLOAD"
	CodeJobPayloadParseError Code = "JOB_PAYLOAD_PARSE_FAILED"
	// CodeJobsUnavailable: ClaimJob found no job ready to run. Not a failure;
	// the worker waits and claims again.
	CodeJobsUnavailable  Code = "JOBS_UNAVAILABLE"
	CodeJobHandlerError  Code = "JOB_HANDLER_ERROR"
	CodeJobEnqueueFailed Code = "JOB_ENQUEUE_FAILED"
	CodeJobClaimFailed   Code = "JOB_CLAIM_FAILED"
	// CodeJobClaimLost: the worker no longer owns the job (the reaper requeued
	// it, or it was cancelled). Stop working on it and don't retry the write.
	CodeJobClaimLost    Code = "JOB_CLAIM_LOST"
	CodeJobUpdateFailed Code = "JOB_UPDATE_FAILED"
	CodeJobLookupFailed Code = "JOB_LOOKUP_FAILED"
	CodeJobNotFound     Code = "JOB_NOT_FOUND"
	// CodeJobEventAppendFailed: writing a job_events row failed.
	CodeJobEventAppendFailed Code = "JOB_EVENT_APPEND_FAILED"
	// CodeJobInvalidState: the job isn't in a state that allows the action,
	// e.g. cancelling a job that already succeeded.
	CodeJobInvalidState Code = "JOB_INVALID_STATE"
)

// Message pipeline job steps
const (
	CodeMessageCheckFailed   Code = "MESSAGE_CHECK_FAILED"
	CodeMessageExtractFailed Code = "MESSAGE_EXTRACT_FAILED"
	CodeMessageVerifyFailed  Code = "MESSAGE_VERIFY_FAILED"
	CodeMessageLookupFailed  Code = "MESSAGE_LOOKUP_FAILED"
	CodeMessagePredictFailed Code = "MESSAGE_PREDICT_FAILED"
	// CodePipelineReportFailed: recording how a message ended failed.
	CodePipelineReportFailed Code = "PIPELINE_REPORT_FAILED"
)

// Workers
const (
	CodeWorkerCreateError Code = "WORKER_CREATE_FAILED"
	CodeWorkerRunError    Code = "WORKER_RUN_FAILED"
)

const (
	CodeCredentialsNotSet Code = "CREDENTIAL_KEY_NOT_SET"
)
