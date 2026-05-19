/**
 * Helper service for interacting with the Pennywise API.
 */
package client

import (
	"context"
	"net/url"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/parser"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/prediction"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

type PennywiseClient struct {
	client *transport.Client
}

type ParsedTransaction struct {
	Amount   float64
	Date     string
	Payee    string
	Account  string
	Category string
}

type Transaction struct {
	ID                    string  `json:"id,omitempty"`
	Date                  string  `json:"date"`
	PayeeId               string  `json:"payeeId"`
	CategoryId            *string `json:"categoryId,omitempty"`
	AccountId             string  `json:"accountId"`
	Amount                float64 `json:"amount"`
	Note                  string  `json:"note"`
	Source                string  `json:"source"` // MLP for prediction, PENNYWISE for frontend
	TransferAccountId     string  `json:"transferAccountId,omitempty"`
	TransferTransactionId string  `json:"transferTransactionId,omitempty"`
}

type PredictionReq struct {
	TransactionId      string  `json:"transactionId"`
	EmailText          string  `json:"emailText"`
	Amount             float64 `json:"amount"`
	Account            string  `json:"account"`
	AccountPrediction  float64 `json:"accountPrediction,omitempty"`
	Payee              string  `json:"payee,omitempty"`
	PayeePrediction    float64 `json:"payeePrediction"`
	Category           string  `json:"category,omitempty"`
	CategoryPrediction float64 `json:"categoryPrediction,omitempty"`
}

// GoogleUserInfo matches the API response from GET /api/auth/google/users
type GoogleUserInfo struct {
	GoogleID        string                            `json:"googleId"`
	OAuthClientType sharedModel.GoogleOAuthClientType `json:"oauthClientType"`
	Email           string                            `json:"email"`
	GmailHistoryID  int                               `json:"gmailHistoryId"`
	RefreshToken    string                            `json:"refreshToken"`
	BudgetID        string                            `json:"budgetId"`
}

type searchResult struct {
	ID string `json:"id"`
}

type updateHistoryRequest struct {
	Email           string                            `json:"email"`
	OAuthClientType sharedModel.GoogleOAuthClientType `json:"oauthClientType"`
	GmailHistoryID  uint64                            `json:"gmailHistoryId"`
}

func NewPennywiseClient(client *transport.Client) *PennywiseClient {
	return &PennywiseClient{client: client}
}

func buildPath(path string, query map[string]string) string {
	if len(query) == 0 {
		return path
	}
	params := url.Values{}
	for k, v := range query {
		params.Add(k, v)
	}
	return path + "?" + params.Encode()
}

// GetUser fetches google user info (including budgetId) by email.
// This endpoint doesn't require budget scoping.
func (s *PennywiseClient) GetUser(ctx context.Context, email string) (*sharedModel.GoogleUserInfo, error) {
	log := logger.Logger(ctx)
	log.Info("getting user by email", "email", email)

	path := buildPath("/api/auth/google/users", map[string]string{"email": email})
	user, err := transport.Get[sharedModel.GoogleUserInfo](ctx, s.client, path)
	if err != nil {
		log.Error("error getting user", "error", err)
		return nil, errs.Wrap(errs.CodeUserLookupFailed, "failed to get user by email", err)
	}
	return &user, nil
}

// UpdateUserHistoryId updates the gmail history ID for a user by email.
// This endpoint doesn't require budget scoping.
func (s *PennywiseClient) UpdateUserHistoryId(
	ctx context.Context,
	email string,
	oauthClientType sharedModel.GoogleOAuthClientType,
	historyId uint64,
) error {
	oauthClientType = sharedModel.NormalizeGoogleOAuthClientType(oauthClientType)
	log := logger.Logger(ctx)
	log.Info("updating user history id", "email", email, "oauthClientType", oauthClientType, "historyId", historyId)

	data := updateHistoryRequest{Email: email, OAuthClientType: oauthClientType, GmailHistoryID: historyId}
	_, err := transport.Patch[map[string]any](ctx, s.client, "/api/auth/google/users", nil, data)
	if err != nil {
		return errs.Wrap(errs.CodeUserUpdateFailed, "failed to update history id", err)
	}
	return nil
}

// CreateTransaction creates a transaction via the pennywise API.
// Requires budget ID in context (set via utils.WithBudgetID).
func (s *PennywiseClient) CreateTransaction(
	ctx context.Context,
	parsedDetails *parser.EmailDetails,
	predictedFields *prediction.PredictedFields,
) (*Transaction, error) {
	log := logger.Logger(ctx)

	txnData := ParsedTransaction{
		Amount:   parsedDetails.Amount,
		Date:     parsedDetails.Date,
		Payee:    predictedFields.Payee.Label,
		Account:  predictedFields.Account.Label,
		Category: predictedFields.Category.Label,
	}
	log.Info("creating transaction", "txnData", txnData)

	// search for account
	accPath := buildPath("/api/accounts/search", map[string]string{"name": txnData.Account})
	accounts, err := transport.Get[[]searchResult](ctx, s.client, accPath)
	if err != nil {
		return nil, errs.Wrap(errs.CodeAccountLookupFailed, "error searching for account", err)
	}
	if len(accounts) == 0 {
		return nil, errs.New(errs.CodeAccountLookupFailed, "Account not found for %s", txnData.Account)
	}
	accountId := accounts[0].ID

	// search for payee
	payeePath := buildPath("/api/payees/search", map[string]string{"name": txnData.Payee})
	payees, err := transport.Get[[]searchResult](ctx, s.client, payeePath)
	if err != nil {
		return nil, errs.Wrap(errs.CodePayeeLookupFailed, "error searching for payee", err)
	}
	if len(payees) == 0 {
		return nil, errs.New(errs.CodePayeeLookupFailed, "Payee not found for %s", txnData.Payee)
	}
	payeeId := payees[0].ID

	// search for category
	var catIdPtr *string
	if txnData.Category != "null" && txnData.Category != "" {
		catPath := buildPath("/api/categories/search", map[string]string{"name": txnData.Category})
		categories, err := transport.Get[[]searchResult](ctx, s.client, catPath)
		if err != nil {
			return nil, errs.Wrap(errs.CodeCategoryLookupFailed, "error searching for category", err)
		}
		if len(categories) == 0 {
			return nil, errs.New(errs.CodeCategoryLookupFailed, "Category not found for %s", txnData.Category)
		}
		catId := categories[0].ID
		catIdPtr = &catId
		log.Info("category found", "categoryId", catId)
	} else {
		log.Info("category is null")
	}

	newTxn := Transaction{
		Date:       txnData.Date,
		Amount:     txnData.Amount,
		AccountId:  accountId,
		PayeeId:    payeeId,
		CategoryId: catIdPtr,
		Source:     "MLP",
		Note:       "",
	}

	txns, err := transport.Post[[]Transaction](ctx, s.client, "/api/transactions", nil, newTxn)
	if err != nil {
		return nil, errs.Wrap(errs.CodeTransactionCreateFailed, "error creating transaction", err)
	}
	if len(txns) == 0 {
		return nil, errs.New(errs.CodeTransactionNotCreated, "No transactions received")
	}
	log.Info("transaction created", "id", txns[0].ID)
	return &txns[0], nil
}

// CreatePrediction creates a prediction record via the pennywise API.
// Requires budget ID in context (set via utils.WithBudgetID).
func (s *PennywiseClient) CreatePrediction(
	ctx context.Context,
	parsedDetails *parser.EmailDetails,
	predictedFields *prediction.PredictedFields,
	txnData *Transaction,
) error {
	predictionReq := PredictionReq{
		TransactionId: txnData.ID,
		Amount:        txnData.Amount,
		EmailText:     parsedDetails.Text,
		Account:       predictedFields.Account.Label,
	}
	if predictedFields.Account.Confidence != -1 {
		predictionReq.AccountPrediction = predictedFields.Account.Confidence
	}
	if predictedFields.Payee.Confidence != -1 {
		predictionReq.Payee = predictedFields.Payee.Label
		predictionReq.PayeePrediction = predictedFields.Payee.Confidence
	}
	if predictedFields.Category.Confidence != -1 {
		predictionReq.Category = predictedFields.Category.Label
		predictionReq.CategoryPrediction = predictedFields.Category.Confidence
	}

	_, err := transport.Post[map[string]any](ctx, s.client, "/api/predictions", nil, predictionReq)
	if err != nil {
		return errs.Wrap(errs.CodePredictionCreateFailed, "error creating prediction", err)
	}
	logger.Logger(ctx).Info("prediction created")
	return nil
}
