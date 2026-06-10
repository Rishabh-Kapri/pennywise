package client

import (
	"context"
	"fmt"
	"strings"

	cfg "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/model"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
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

const extractionModel = "gemma4:12b"

// const extractionPrompt = `You are a financial data extractor. Output strictly JSON.
// RULE: Remove the extra invoice number.
// SCHEMA: {"merchant": "string", "amount": float, "account_card": "string (Bank name and last 4 digits only, no extra words)"}
//
// EXAMPLES:
// Input: "Alert: Rs 500 debited from HDFC CC XX1234 towards SWIGGY"
// Output: {"merchant": "SWIGGY", "amount": 500.0, "account_card": "HDFC 1234"}
//
// Input: "Txn of INR 1540 on ICICI XX4444 at RAZORPAY* MAKE MY T"
// Output: {"merchant": "RAZORPAY* MAKE MY T", "amount": 1540.0, "account_card": "ICICI 4444"}
//
// Input: "UPDATE: Your A/C XXXXXX1234 is debited by Rs 45.00 on 15-Apr-26 for Swiggy Genie via PTM*BUNDLE TECHNOL. Clear Bal Rs 12,345.67."
// Output: {"merchant": "Swiggy Genie", "amount": 45.0, "account_card": "1234"}
//
// Input: "Dear Customer,\nRs.1000.00 has been debited from account 1234 to VPA johndoes@okicici HOTEL JOE AND JOHN on 29-10-25."
// Output: {"merchant": "johndoes@okicici HOTEL JOE AND JOHN", "amount": 1000.0, "account_card": "1234"}
//
// Input: "Dear Customer,\nRs. 15000.00 is successfully credited to your account **9999 by VPA userhigh@okhdfcbank USER HIGH on 07-10-25."
// Output: {"merchant": "userhigh@okhdfcbank USER HIGH", "amount": 15000.0, "account_card": "9999"}
//
// Now process this input:
// Input: "`
//
// Input: "Dear Customer, Thank you for banking with HDFC Bank. Amount deducted Of Rs. 39,090.00 From your SBI Bank A/c XX1234 For NEFT transaction Via SBI Bank Online Banking. Not you? Call 18002586161 Warm Regards, HDFC BankFor more details on Service charges and Fees, click here.. © HDFC Bank"
// Output: {"merchant": "", "amount": -39090.0, "account_card": "1234"}
const EmailSummarizationPrompt = `You are a bank email summarizer. Summarize the transaction in one sentence. Return only the sentence, nothing else.
If not a transaction email, return empty string "".

EXAMPLES:
Input: "Dear Customer, We would like to inform you that Rs. 939.00 has been debited from your HDFC Bank Credit Card ending 4432 towards BEMINIMALIST on 29 May, 2026 at 21:04:11. To check your balance: https://mycards.hdfc.bank.in Important Note: Call 1800 258 6161. Warm Regards, HDFC Bank."
	Output: { summary: "Rs. 939.00 debited from Credit Card 4432 to BEMINIMALIST on 29 May 2026." }

Input: "Dear Customer, Rs.500.00 has been debited from account 4567 to VPA 9876543210@ybl (JOHN DOE S O JAMES DOE) on 14-07-25. UPI ref: 123456789012. If you did not authorize call 18002586161. Warm Regards, HDFC Bank."
Output: { "summary": "Rs. 500.00 debited from account 4567 to 9876543210@ybl JOHN DOE on 14-07-25." }

Input: "Dear Customer, Rs. 3000.00 is successfully credited to your account 4567 by VPA 9876543210@ybl JOHN DOE on 12-07-25. UPI ref: 987654321098. Warm Regards, HDFC Bank."
Output: { "summary": "Rs. 3000.00 credited to account 4567 from 9876543210@ybl JOHN DOE on 12-07-25." }

Input: "Dear Customer, Thank you for using your HDFC Bank Card XX1234 for Rs. 650.0 at GOOGLEPLAY on 14-05-2026."
Output: { "summary": "Rs. 650.0 debited from Card 1234 at GOOGLEPLAY on 14-05-2026." }

Input: "Dear Customer, Rs.250.00 is debited from your account ending 1234 towards VPA 9997181976-1@okbizaxis (SPARSH Physiotherapy and Wellness Center) on 30-05-26. UPI ref: 901444291862."
Output: { "summary": "Rs. 250.00 debited from account 1234 to 9997181976-1@okbizaxis SPARSH Physiotherapy and Wellness Center on 30-05-26." }

Input: "Dear Customer, Rs.900.00 has been successfully credited to your HDFC Bank account ending in 8936. Transaction Details: a. Date: 26-05-26 b. Sender: ANSHUL WAGADRE (VPA: 7999470042@yescred) c. UPI Reference No.: 651206303881. Need Help? India (Toll-Free): 1800 258 6161."
Output: { "summary": "Rs. 900.00 credited to account 8936 from 7999470042@yescred ANSHUL WAGADRE on 26-05-26." }

Input: "Dear Customer, You have successfully added a payee ITD with A/c XX5116 to your HDFC Bank Account via Online Banking. Not you? Call 18002586161."
Output: { "summary": "" }

Input: `

const extractionPrompt = `You are a financial data extractor. You will receive email text and you need to output strictly JSON. Do not wrap the response in markdown blocks.
	RULE: 
	- Remove the extra invoice number.
	- Add negative sign if the amount is debited.
	- When merchant name is empty or not found, use empty string as merchant name.
	- When the email is not a transaction email, return empty JSON.
	- Skip the e-mandate email.
	- Output field names must match the schema exactly. Do not rename, add, or omit any fields.
	- date is always required if present in the email. Date formats like "3 Apr, 2023" must be parsed as 2023-04-03.
	SCHEMA: {"merchant": "string", "amount": float, "date": "string (formatted as YYYY-MM-DD)", "time": string | null, "account_card": "string (Bank name and last 4 digits only, no extra words)", "reasoning": "string (Brief 1 sentence explanation of why this classification is chosen)"}

	EXAMPLES: 
  Input: "Dear Customer, Rs.500.00 has been debited from account 4567 to VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 14-07-25. Your UPI transaction reference number is 123456789012. If you did not authorize this transaction, please report it immediately by calling 18002586161 Or SMS BLOCK UPI to 7308080808. Warm Regards, HDFC BankFor more details on Service charges and Fees, click here.. © HDFC Bank"
	Output: {"merchant": "JOHN DOE S O JAMES DOE", "amount": -500.0, "date": "2025-07-14", time: null, "account_card": "4567", "reasoning": "The transaction is a debit of 500.00 from an HDFC Bank acocunt ending in 4567 for the merchant JOHN DOE SO JAMES DOE."}

	Input: "Dear Customer, Rs. 3000.00 is successfully credited to your account **4567 by VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 12-07-25. Your UPI transaction reference number is 987654321098. Thank you for banking with us. Warm Regards, HDFC BankFor more details on Service charges and Fees, click here.. © HDFC Bank}"
	Output: {"merchant": "JOHN DOE S O JAMES DOE", "amount": 3000.0, "date": "2025-07-12", "account_card": "4567", "reasoning": "The transaction is a credit of 500.00 to HDFC Bank account Card ending in 4432 from the merchant JOHN DOE SO JAMES DOE."}

	Input: "Dear Customer, Rs.1200.00 has been debited from your HDFC Bank Credit Card ending 9876 towards NETFLIX on 05 Jan, 2025 at 10:22:11."
	Output: {"merchant": "NETFLIX", "amount": -1200.0, "date": "2025-01-05", "time": "10:22:11", "account_card": "HDFC 9876", "reasoning": "Debit from HDFC credit card ending 9876 to Netflix."}

	Input: "Dear Customer, Greetings from HDFC Bank! Your Canva Pty Ltd bill, set up through E-mandate (Auto payment), has been successfully paid using your HDFC Bank Credit Card ending 1234. Transaction Details: Amount: INR 500.00 Date: 10/06/2026 SI Hub ID: Y8Inwhnbjn To manage your e-Mandates, please visit: https://www.sihub.in/managesi/hdfcbank Thank you for banking with us. Warm regards, HDFC BankFor more details on Service charges and Fees, click here.. © HDFC Bank"
	Output: {"merchant": "", "amount": 0.0, "date": "", "time": "", "account_card": "", "reasoning": "Skipping e-mandate payment from HDFC credit card ending 1234"}

	Now process this input:
	Input: `

// ExtractEmailData implements Phase 1 of the classification pipeline:
// sends raw email text to a local SLM (Gemma via Ollama) in JSON mode
// to extract structured {merchant, amount, account_card} from chaotic bank alerts.
func (c *OllamaClient) ExtractEmailData(
	ctx context.Context,
	rawText string,
) (*sharedModel.ExtractedEmailResponse, error) {
	prompt := extractionPrompt + rawText + "\"\nOutput:"

	extracted, err := GenericLLMCall[sharedModel.ExtractedEmailResponse](ctx, c, model.PromptReq{
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

	return c.doLocalGenerate(ctx, ollamaModel, prompt, nil)
}

// GenericLLMCall sends a prompt and unmarshals the JSON response into type T.
// Routes to OpenAI or local Ollama based on the model prefix ("openai/...").
func GenericLLMCall[T any](ctx context.Context, c *OllamaClient, req model.PromptReq) (T, error) {
	var jsonStr string
	var err error

	if strings.HasPrefix(req.Model, "openai") {
		jsonStr, err = c.doOpenAI(ctx, req)
	} else {
		jsonStr, err = c.doLocalGenerate(ctx, req.Model, req.Prompt, req.Temperature)
	}

	var zero T
	if err != nil {
		return zero, err
	}
	return utils.UnmarshalResponse[T]([]byte(jsonStr))
}

// ── Internal dispatch ───────────────────────────────────────────

// doLocalGenerate calls the local Ollama /api/generate endpoint with deterministic settings.
func (c *OllamaClient) doLocalGenerate(
	ctx context.Context,
	ollamaModel string,
	prompt string,
	temperature *float32,
) (string, error) {
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

	modelTemp := float32(0.0)
	if temperature != nil {
		modelTemp = *temperature
	}

	resp, err := transport.Post[model.OllamaResponse](ctx, c.client, "/api/generate", nil, model.OllamaRequest{
		Model:  ollamaModel,
		Prompt: prompt,
		Format: "json",
		Stream: false,
		Options: map[string]any{
			"temperature": modelTemp,
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
