package errs

import "fmt"

type Code string

const (
	CodeInvalidArgument           Code = "INVALID_ARGUMENT"
	CodeTransactionCreateFailed   Code = "TRANSACTION_CREATE_FAILED"
	CodeTransactionNotCreated     Code = "TRANSACTION_NOT_CREATED"
	CodeTransactionUpdateFailed   Code = "TRANSACTION_UPDATE_FAILED"
	CodeTransactionLookupFailed   Code = "TRANSACTION_LOOKUP_FAILED"
	CodeTransactionDeleteFailed   Code = "TRANSACTION_DELETE_FAILED"
	CodePayeeLookupFailed         Code = "PAYEE_LOOKUP_FAILED"
	CodeAccountLookupFailed       Code = "ACCOUNT_LOOKUP_FAILED"
	CodeTransferCreateFailed      Code = "TRANSFER_CREATE_FAILED"
	CodeTransferNotCreated        Code = "TRANSFER_NOT_CREATED"
	CodeTransferLinkFailed        Code = "TRANSFER_LINK_FAILED"
	CodeBudgetLookupFailed        Code = "BUDGET_LOOKUP_FAILED"
	CodeMonthlyBudgetLookupFailed Code = "MONTHLY_BUDGET_LOOKUP_FAILED"
	CodeMonthlyBudgetCreateFailed Code = "MONTHLY_BUDGET_CREATE_FAILED"
	CodeMonthlyBudgetUpdateFailed Code = "MONTHLY_BUDGET_UPDATE_FAILED"
)

type Error struct {
	Code    Code
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

// Unwrap returns the underlying error, if any.
func (e *Error) Unwrap() error {
	return e.Err
}

// New creates a fresh error with no underlying cause.
func New(code Code, message string, args ...any) error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(message, args...),
	}
}

// Wrap creates an error that contains another error as its cause.
func Wrap(code Code, message string, err error) error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
