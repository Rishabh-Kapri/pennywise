package service

// Prompts for the email flow the Temporal activities run (internal/temporal).
// The job pipeline's prompts are in adapters/llm.

// emailSummarizationPrompt turns an email into the one-line summary shown on
// the prediction.
const emailSummarizationPrompt = `You are a bank email summarizer. Summarize the transaction in one sentence. Return only the sentence, nothing else.
If not a transaction email, return empty string "".

EXAMPLES:
Input: "Dear Customer, We would like to inform you that Rs. 939.00 has been debited from your HDFC Bank Credit Card ending 2222 towards BEMINIMALIST on 29 May, 2026 at 21:04:11. To check your balance: https://mycards.hdfc.bank.in Important Note: Call 1800 258 6161. Warm Regards, HDFC Bank."
	Output: { summary: "Rs. 939.00 debited from Credit Card 2222 to BEMINIMALIST on 29 May 2026." }

Input: "Dear Customer, Rs.500.00 has been debited from account 4567 to VPA 9876543210@ybl (JOHN DOE S O JAMES DOE) on 14-07-25. UPI ref: 123456789012. If you did not authorize call 18002586161. Warm Regards, HDFC Bank."
Output: { "summary": "Rs. 500.00 debited from account 4567 to 9876543210@ybl JOHN DOE on 14-07-25." }

Input: "Dear Customer, Rs. 3000.00 is successfully credited to your account 4567 by VPA 9876543210@ybl JOHN DOE on 12-07-25. UPI ref: 987654321098. Warm Regards, HDFC Bank."
Output: { "summary": "Rs. 3000.00 credited to account 4567 from 9876543210@ybl JOHN DOE on 12-07-25." }

Input: "Dear Customer, Thank you for using your HDFC Bank Card XX1234 for Rs. 650.0 at GOOGLEPLAY on 14-05-2026."
Output: { "summary": "Rs. 650.0 debited from Card 1234 at GOOGLEPLAY on 14-05-2026." }

Input: "Dear Customer, Rs.250.00 is debited from your account ending 1234 towards VPA 9000000002-1@okbizaxis (SAMPLE Clinic and Wellness Center) on 30-05-26. UPI ref: 901444291862."
Output: { "summary": "Rs. 250.00 debited from account 1234 to 9000000002-1@okbizaxis SAMPLE Clinic and Wellness Center on 30-05-26." }

Input: "Dear Customer, Rs.900.00 has been successfully credited to your HDFC Bank account ending in 1111. Transaction Details: a. Date: 26-05-26 b. Sender: SAMPLE PERSON (VPA: 9000000103@yescred) c. UPI Reference No.: 651206303881. Need Help? India (Toll-Free): 1800 258 6161."
Output: { "summary": "Rs. 900.00 credited to account 1111 from 9000000103@yescred SAMPLE PERSON on 26-05-26." }

Input: "Dear Customer, You have successfully added a payee ITD with A/c XX7004 to your HDFC Bank Account via Online Banking. Not you? Call 18002586161."
Output: { "summary": "" }

Input: `

// EmailExtractionPrompt reads amount, merchant, account and date from an email.
// cmd/backfill sends it to Ollama too.
const EmailExtractionPrompt = `You are a financial data extractor. You will receive email text and you need to output strictly JSON. Do not wrap the response in markdown blocks.
	RULE: 
	- Remove the extra invoice number.
	- Add negative sign if the amount is debited.
	- When merchant name is empty or not found, use empty string as merchant name.
	- When the email is not a transaction email, return empty JSON.
	- Skip the e-mandate email.
	- Output field names must match the schema exactly. Do not rename, add, or omit any fields.
	- date is always required if present in the email. Date formats like "3 Apr, 2023" must be parsed as 2023-04-03.

	## Output shapes
	### Shape A — Extracted transaction
	Use when the email represents a completed debit or credit.

	{
		"merchant": "Google Play",
		"amount": -650.00,
		"date": "2026-06-14",
		"time": "10:32",
		"account_card": "HDFC 1234",
		"reasoning": "..."
	}

	reasoning should be 1–2 sentences explaining how key fields were extracted or
	any ambiguity resolved. Examples:
	- "Debit of ₹650 to Google Play from HDFC Credit Card ending 1234 dated 14/06/2026."
	- "Merchant extracted from POS label 'AMZN MKTP IN', normalized to Amazon. Amount negative as it is a debit."

	### Shape B — Skipped email
	Use when the email is an e-mandate, OTP, marketing, or otherwise not a completed transaction.

	{
		"merchant": "",
		"amount": 0.0,
		"date": "",
		"time": "",
		"account_card": "",
		"skipped": true,
		"reasoning": "Skipping e-mandate payment from HDFC credit card ending 1234"
	}

	Do NOT use Shape A with empty strings or 0.0 as a skip signal. Always use Shape B.

	EXAMPLES: 
  Input: "Dear Customer, Rs.500.00 has been debited from account 4567 to VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 14-07-25. Your UPI transaction reference number is 123456789012. If you did not authorize this transaction, please report it immediately by calling 18002586161 Or SMS BLOCK UPI to 9000000205. Warm Regards, HDFC BankFor more details on Service charges and Fees, click here.. © HDFC Bank"
	Output: {"merchant": "JOHN DOE S O JAMES DOE", "amount": -500.0, "date": "2025-07-14", time: "", "account_card": "4567", "reasoning": "The transaction is a debit of 500.00 from an HDFC Bank acocunt ending in 4567 for the merchant JOHN DOE SO JAMES DOE."}

	Input: "Dear Customer, Rs. 3000.00 is successfully credited to your account **4567 by VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 12-07-25. Your UPI transaction reference number is 987654321098. Thank you for banking with us. Warm Regards, HDFC BankFor more details on Service charges and Fees, click here.. © HDFC Bank}"
	Output: {"merchant": "JOHN DOE S O JAMES DOE", "amount": 3000.0, "date": "2025-07-12", "time": "", "account_card": "4567", "reasoning": "The transaction is a credit of 500.00 to HDFC Bank account Card ending in 2222 from the merchant JOHN DOE SO JAMES DOE."}

	Input: "Dear Customer, Rs.1200.00 has been debited from your HDFC Bank Credit Card ending 9876 towards NETFLIX on 05 Jan, 2025 at 10:22:11."
	Output: {"merchant": "NETFLIX", "amount": -1200.0, "date": "2025-01-05", "time": "10:22:11", "account_card": "HDFC 9876", "reasoning": "Debit from HDFC credit card ending 9876 to Netflix."}

	Input: "Dear Customer, Greetings from HDFC Bank! Your Canva Pty Ltd bill, set up through E-mandate (Auto payment), has been successfully paid using your HDFC Bank Credit Card ending 1234. Transaction Details: Amount: INR 500.00 Date: 10/06/2026 SI Hub ID: Y8Inwhnbjn To manage your e-Mandates, please visit: https://www.sihub.in/managesi/hdfcbank Thank you for banking with us. Warm regards, HDFC BankFor more details on Service charges and Fees, click here.. © HDFC Bank"
	Output: {"merchant": "", "amount": 0.0, "date": "", "time": "", "account_card": "", "reasoning": "Skipping e-mandate payment from HDFC credit card ending 1234", "skipped": true}

	Now process this input:
	Input: `

// emailExtractionSchema is the Ollama structured-outputs JSON schema for EmailExtractionPrompt.
// It grammar-constrains generation so the model can never reply with an empty
// string or a bare array for non-transaction emails — every field is forced to
// exist, and skips must use Shape B (skipped: true).
var emailExtractionSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"merchant":     map[string]any{"type": "string"},
		"amount":       map[string]any{"type": "number"},
		"date":         map[string]any{"type": "string"},
		"time":         map[string]any{"type": "string"},
		"account_card": map[string]any{"type": "string"},
		"reasoning":    map[string]any{"type": "string"},
		"skipped":      map[string]any{"type": "boolean"},
	},
	"required": []string{"merchant", "amount", "date", "time", "account_card", "reasoning", "skipped"},
}

// predictionPrompt picks a payee and one of the budget's {categories}.
const predictionPrompt = `
You are an expert financial data extraction API. Your job is to analyze raw bank transaction text and output strictly valid JSON.

Extract the clean merchant brand name and categorize the transaction into exactly ONE of the allowed categories.

RULES:
1. MERCHANT NAME: Extract the core brand. Remove all bank jargon (UPI, POS, REF, VPA), dates, and reference numbers. (e.g., "PYU*Swiggy Food 12-Apr" -> "Swiggy").
2. CATEGORY: You must select exactly one category from the ALLOWED CATEGORIES list. If you are completely unsure, use "Uncategorized".
3. SUBSCRIPTIONS: Flag is_subscription as true ONLY if the text implies a recurring payment (e.g., Netflix, Spotify, AWS, "recurring", "mandate").
4. JSON ONLY: Do not wrap the response in markdown blocks. Return only the raw JSON object.
5. Never output the data from the examples. Only process the provided input.

ALLOWED CATEGORIES:
{categories}

EXPECTED JSON SCHEMA:
{
  "merchantName": "string",
  "suggestedTag": "string",
  "confidence": integer (0-100),
  "reasoning": "string (Brief 1-sentence explanation of why you chose this category)"
}
	`
