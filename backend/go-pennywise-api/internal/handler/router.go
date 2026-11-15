package handler

import (
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Handlers is every handler the API serves. Each registers its own routes.
type Handlers struct {
	Account              *AccountHandler
	Agent                *AgentHandler
	AIModel              *AIModelHandler
	AIProvider           *AIProviderHandler
	APIKey               *APIKeyHandler
	Auth                 *AuthHandler
	Budget               *BudgetHandler
	Category             *CategoryHandler
	CategoryGoal         *CategoryGoalHandler
	CategoryGroup        *CategoryGroupHandler
	Demo                 *DemoHandler // nil unless DEMO_MODE is on
	Device               *DeviceHandler
	Document             *DocumentHandler
	Geocode              *GeocodeHandler
	Job                  *JobHandler
	JobSchedule          *JobScheduleHandler
	LoanMetadata         *LoanMetadataHandler
	MailConnection       *MailConnectionHandler
	Onboarding           *OnboardingHandler
	Payee                *PayeeHandler
	PipelineStep         *PipelineStepHandler
	Prediction           *PredictionHandler
	PredictionReview     *PredictionReviewHandler
	RecurringTransaction *RecurringTransactionHandler
	Report               *ReportHandler
	Sms                  *SmsHandler
	Statement            *StatementHandler
	Tag                  *TagHandler
	Transaction          *TransactionHandler
	User                 *UserHandler
	Websocket            *WebsocketHandler
}

// Middleware is what the route groups run before a handler.
type Middleware struct {
	Authenticate gin.HandlerFunc
	RateLimit    gin.HandlerFunc
	// Budget reads X-Budget-ID and checks the caller owns it.
	Budget gin.HandlerFunc
}

// Register mounts every route. There are three kinds: public (login), user
// (signed in, not tied to a budget) and budget (signed in, X-Budget-ID). An
// API key's scope follows the request method; see middleware.ScopeByMethod.
func Register(router gin.IRouter, h Handlers, mw Middleware) {
	scope := middleware.ScopeByMethod()

	h.Websocket.Routes(router.Group("/ws", mw.Authenticate, mw.RateLimit, mw.Budget, scope))

	api := router.Group("/api")
	api.GET("", func(c *gin.Context) { c.String(http.StatusOK, "Health OK!") })
	user := api.Group("", mw.Authenticate, mw.RateLimit, scope)
	budget := api.Group("", mw.Authenticate, mw.RateLimit, mw.Budget, scope)

	h.Auth.Routes(api, user)
	if h.Demo != nil {
		h.Demo.Routes(api)
	}

	h.APIKey.Routes(user)
	h.Budget.Routes(user)
	h.Device.Routes(user)
	h.Geocode.Routes(user)

	h.Account.Routes(budget)
	h.Agent.Routes(budget)
	h.AIModel.Routes(budget)
	h.AIProvider.Routes(budget)
	h.Category.Routes(budget)
	h.CategoryGoal.Routes(budget)
	h.CategoryGroup.Routes(budget)
	h.Document.Routes(budget)
	h.Job.Routes(budget)
	h.JobSchedule.Routes(budget)
	h.LoanMetadata.Routes(budget)
	h.MailConnection.Routes(budget)
	h.Onboarding.Routes(budget)
	h.Payee.Routes(budget)
	h.PipelineStep.Routes(budget)
	h.Prediction.Routes(budget)
	h.PredictionReview.Routes(budget)
	h.RecurringTransaction.Routes(budget)
	h.Report.Routes(budget)
	h.Sms.Routes(budget)
	h.Statement.Routes(budget)
	h.Tag.Routes(budget)
	h.Transaction.Routes(budget)
	h.User.Routes(budget)
}
