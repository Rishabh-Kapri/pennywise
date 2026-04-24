package client

import (
	"context"
	"fmt"
	"strings"

	cfg "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/model"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// ── Client ──────────────────────────────────────────────────────

type OllamaClient struct {
	client *transport.Client
	config cfg.Config
	tracer oteltrace.Tracer
}

func NewOllamaClient(c *transport.Client, tracer oteltrace.Tracer) *OllamaClient {
	return &OllamaClient{client: c, config: cfg.Load(), tracer: tracer}
}

// ── Phase 1: Email data extraction ──────────────────────────────

const extractionModel = "gemma4"

const extractionPrompt = `You are a financial data extractor. Output strictly JSON.
RULE: Remove the extra invoice number.
SCHEMA: {"merchant": "string", "amount": float, "account_card": "string (Bank name and last 4 digits only, no extra words)"}

EXAMPLES:
Input: "Alert: Rs 500 debited from HDFC CC XX1234 towards SWIGGY"
Output: {"merchant": "SWIGGY", "amount": 500.0, "account_card": "HDFC 1234"}

Input: "Txn of INR 1540 on ICICI XX4444 at RAZORPAY* MAKE MY T"
Output: {"merchant": "RAZORPAY* MAKE MY T", "amount": 1540.0, "account_card": "ICICI 4444"}

Input: "UPDATE: Your A/C XXXXXX1234 is debited by Rs 45.00 on 15-Apr-26 for Swiggy Genie via PTM*BUNDLE TECHNOL. Clear Bal Rs 12,345.67."
Output: {"merchant": "Swiggy Genie", "amount": 45.0, "account_card": "1234"}

Input: "Dear Customer,\nRs.1000.00 has been debited from account 1234 to VPA johndoes@okicici HOTEL JOE AND JOHN on 29-10-25."
Output: {"merchant": "johndoes@okicici HOTEL JOE AND JOHN", "amount": 1000.0, "account_card": "1234"}

Input: "Dear Customer,\nRs. 15000.00 is successfully credited to your account **9999 by VPA userhigh@okhdfcbank USER HIGH on 07-10-25."
Output: {"merchant": "userhigh@okhdfcbank USER HIGH", "amount": 15000.0, "account_card": "9999"}

Now process this input:
Input: "`

// ExtractEmailData implements Phase 1 of the classification pipeline:
// sends raw email text to a local SLM (Gemma via Ollama) in JSON mode
// to extract structured {merchant, amount, account_card} from chaotic bank alerts.
func (c *OllamaClient) ExtractEmailData(ctx context.Context, rawText string) (*model.ExtractedEmail, error) {
	prompt := extractionPrompt + rawText + "\"\nOutput:"

	extracted, err := GenericLLMCall[model.ExtractedEmail](ctx, c, model.PromptReq{
		Model:  extractionModel,
		Prompt: prompt,
	})
	if err != nil {
		return nil, fmt.Errorf("phase 1 extraction failed: %w", err)
	}

	return &extracted, nil
}

// ── Embedding ───────────────────────────────────────────────────

// Embed generates a vector embedding for the given text using the specified model (e.g., bge-m3).
func (c *OllamaClient) Embed(ctx context.Context, ollamaModel string, text string) ([]float64, error) {
	ctx, span := c.tracer.Start(ctx, "embed "+ollamaModel,
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()
	span.SetAttributes(
		attribute.String("gen_ai.system", "ollama"),
		attribute.String("gen_ai.request.model", ollamaModel),
		attribute.String("gen_ai.prompt", text),
	)
	resp, err := transport.Post[model.EmbedResponse](ctx, c.client, "/api/embed", nil, model.EmbedRequest{
		Model: ollamaModel,
		Input: text,
	})
	if err != nil {
		err := errs.Wrap(errs.CodeInternalError, "ollama embed", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if len(resp.Embeddings) == 0 {
		err := errs.New(errs.CodeInternalError, "ollama embed: no embeddings returned")
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(attribute.Int("gen_ai.usage.output_tokens", len(resp.Embeddings[0])))
	return resp.Embeddings[0], nil
}

// ── Text generation ─────────────────────────────────────────────

// Generate sends a prompt to the LLM and returns the raw JSON response string.
// Routes to OpenAI or local Ollama based on the model prefix ("openai/...").
// For OpenAI calls, cleanedText and amount are formatted into the user message.
func (c *OllamaClient) Generate(
	ctx context.Context,
	ollamaModel string,
	prompt string,
	cleanedText string,
	amount float64,
) (string, error) {
	if strings.HasPrefix(ollamaModel, "openai") {
		userPrompt := fmt.Sprintf("Transaction: \"%s\"\nAmount: %.2f", cleanedText, amount)
		return c.doOpenAI(ctx, model.PromptReq{Model: ollamaModel, Prompt: prompt, UserPrompt: userPrompt})
	}

	return c.doLocalGenerate(ctx, ollamaModel, prompt)
}

// GenericLLMCall sends a prompt and unmarshals the JSON response into type T.
// Routes to OpenAI or local Ollama based on the model prefix ("openai/...").
func GenericLLMCall[T any](ctx context.Context, c *OllamaClient, req model.PromptReq) (T, error) {
	var jsonStr string
	var err error

	if strings.HasPrefix(req.Model, "openai") {
		jsonStr, err = c.doOpenAI(ctx, req)
	} else {
		jsonStr, err = c.doLocalGenerate(ctx, req.Model, req.Prompt)
	}

	var zero T
	if err != nil {
		return zero, err
	}
	return utils.UnmarshalResponse[T]([]byte(jsonStr))
}

// ── Internal dispatch ───────────────────────────────────────────

// doLocalGenerate calls the local Ollama /api/generate endpoint with deterministic settings.
func (c *OllamaClient) doLocalGenerate(ctx context.Context, ollamaModel string, prompt string) (string, error) {
	ctx, span := c.tracer.Start(ctx, "chat "+ollamaModel,
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()
	// Set GenAI attributes that Langfuse understands
	span.SetAttributes(
		attribute.String("gen_ai.system", "ollama"),
		attribute.String("gen_ai.request.model", ollamaModel),
		attribute.Float64("gen_ai.request.temperature", 0.0),
		attribute.Float64("gen_ai.request.top_p", 1.0),
		attribute.String("gen_ai.prompt", prompt),
	)

	resp, err := transport.Post[model.OllamaResponse](ctx, c.client, "/api/generate", nil, model.OllamaRequest{
		Model:  ollamaModel,
		Prompt: prompt,
		Format: "json",
		Stream: false,
		Options: map[string]any{
			"temperature": 0.0,
			"top_p":       1.0,
		},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", errs.Wrap(errs.CodeInternalError, "ollama generate", err)
	}

	span.SetAttributes(
		attribute.String("gen_ai.completion", resp.Response),
		attribute.Int("gen_ai.usage.input_tokens", resp.PromptEvalCount),
		attribute.Int("gen_ai.usage.output_tokens", resp.EvalCount),
	)
	return resp.Response, nil
}

const openaiEndpoint = "https://api.openai.com/v1/chat/completions"

// doOpenAI calls the OpenAI chat completions API.
// If req.UserPrompt is set, Prompt is the system message and UserPrompt is the user message.
// Otherwise, Prompt is sent as a single user message.
func (c *OllamaClient) doOpenAI(ctx context.Context, req model.PromptReq) (string, error) {
	log := logger.Logger(ctx)
	// Start a OTEL trace
	openAIModel := strings.TrimPrefix(req.Model, "openai/")
	ctx, span := c.tracer.Start(ctx, "chat "+openAIModel,
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()
	// Set GenAI attributes that Langfuse understands
	span.SetAttributes(
		attribute.String("gen_ai.system", "openai"),
		attribute.String("gen_ai.request.model", openAIModel),
	)
	// Set prompt as input
	if req.UserPrompt != "" {
		span.SetAttributes(
			attribute.String("gen_ai.prompt", req.Prompt+"\n---\n"+req.UserPrompt),
		)
	} else {
		span.SetAttributes(
			attribute.String("gen_ai.prompt", req.Prompt),
		)
	}

	headers := map[string][]string{
		"Authorization": {fmt.Sprintf("Bearer %s", c.config.OpenAIAPIKey)},
	}

	var messages []model.OpenAIMessage
	if req.UserPrompt != "" {
		messages = []model.OpenAIMessage{
			{Role: "system", Content: req.Prompt},
			{Role: "user", Content: req.UserPrompt},
		}
	} else {
		messages = []model.OpenAIMessage{
			{Role: "user", Content: req.Prompt},
		}
	}

	reqData := model.OpenAIRequest{
		Model:    openAIModel,
		Messages: messages,
	}
	reqData.ResponseFormat.Type = "json_object"

	resp, err := transport.Post[model.OpenAIResponse](ctx, c.client, openaiEndpoint, headers, reqData)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", errs.Wrap(errs.CodeInternalError, "openai generate", err)
	}
	if len(resp.Choices) == 0 {
		err := errs.New(errs.CodeInternalError, "openai: no choices returned")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}
	// Record completion + token usage (Langfuse reads these for cost tracking)
	span.SetAttributes(
		attribute.String("gen_ai.completion", resp.Choices[0].Message.Content),
		attribute.String("gen_ai.response.model", resp.Model),
		attribute.Int("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
		attribute.Int("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
	)

	log.Debug("openai generate", "model", openAIModel, "resp", resp.Choices[0].Message.Content)
	return resp.Choices[0].Message.Content, nil
}
