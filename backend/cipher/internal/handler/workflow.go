package handler

import (
	"net/http"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/gin-gonic/gin"
	tc "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

type WorkflowHandler interface {
	RetryPredict(c *gin.Context)
	StartParsedEmailToTransaction(c *gin.Context)
}

type workflowHandler struct {
	temporalClient tc.Client
}

func NewWorkflowHandler(tc tc.Client) WorkflowHandler {
	return &workflowHandler{temporalClient: tc}
}

// StartParsedEmailToTransaction triggers the ParsedEmailToTransactionWorkflow which
// runs steps 2-4 (Predict -> CreateTransaction -> CreateCipherPrediction) skipping
// the Gmail fetch step.
//
// POST /api/workflows/parsed-to-transaction
func (h *workflowHandler) StartParsedEmailToTransaction(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.Logger(ctx)

	var input sharedModel.ParsedEmailsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Error("error unmarshalling request body", "error", err)
		wrappedErr := errs.Wrap(errs.CodeInternalError, "error unmarshalling request body", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": wrappedErr.Error()})
		return
	}
	log.Info("ParsedEmailToTransaction", "input", input)

	we, err := h.temporalClient.ExecuteWorkflow(
		ctx,
		tc.StartWorkflowOptions{
			TaskQueue: sharedModel.PennywiseTaskQueue,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1,
			},
		},
		sharedModel.ParsedEmailToTransactionWorkflowName,
		input,
	)
	if err != nil {
		log.Error("error starting parsed-email-to-transaction workflow", "error", err)
		wrappedErr := errs.Wrap(errs.CodeInternalError, "error starting workflow", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": wrappedErr.Error()})
		return
	}

	log.Info("parsed-email-to-transaction workflow started", "workflowId", we.GetID(), "runId", we.GetRunID())
	c.JSON(http.StatusOK, gin.H{"workflowId": we.GetID(), "runId": we.GetRunID()})
}

// RetryPredict sends a retry-predict signal to a parked EmailToTransactionWorkflow.
// Use this when Ollama was unavailable and the workflow is waiting for a manual nudge.
//
// POST /api/workflows/:workflowId/retry-predict
func (h *workflowHandler) RetryPredict(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.Logger(ctx)

	workflowId := c.Param("workflowId")
	if workflowId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflowId is required"})
		return
	}

	err := h.temporalClient.SignalWorkflow(ctx, workflowId, "", sharedModel.RetryPredictSignal, nil)
	if err != nil {
		log.Error("error sending retry-predict signal", "error", err, "workflowId", workflowId)
		wrappedErr := errs.Wrap(errs.CodeInternalError, "error sending retry-predict signal", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": wrappedErr.Error()})
		return
	}

	log.Info("retry-predict signal sent", "workflowId", workflowId)
	c.JSON(http.StatusOK, gin.H{"status": "signal sent", "workflowId": workflowId})
}
