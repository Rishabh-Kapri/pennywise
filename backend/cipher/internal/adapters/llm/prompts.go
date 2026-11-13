package llmAdapter

// The prompts this adapter sends. Each asks for the JSON its method parses.

// extractionPrompt asks a chat model for an extraction (see Extract).
const extractionPrompt = `You are a financial data extractor. You will receive email text and you need to output strictly JSON. Do not wrap the response in markdown blocks.
	RULE: 
	- Remove the extra invoice number.
	- Add negative sign if the amount is debited.
	- When merchant name is empty or not found, use empty string as merchant name.
	- Return every field blank, {"account": "", "merchant": "", "currency": "", "amount": 0.0, "date": "", "time": ""}, only for an email with no amount, an OTP, a reminder or upcoming payment, or a card e-mandate (Auto payment) notice or confirmation like the last example.
	- A reversal or refund counts even when it "has been initiated" or will take time to reflect. It is money coming in, so the amount is positive and the merchant is the one it came from.
	- A debit from a bank account "with UMRN" is a completed NACH debit, not an e-mandate notice. Extract it, and the merchant is the name before any / and number.
	- Output field names must match the schema exactly. Do not rename, add, or omit any fields.
	- date is always required if present in the email. Always write it as YYYY-MM-DD: "3 Apr, 2023" becomes 2023-04-03, "05-Nov-2026" becomes 2026-11-05, and numeric dates are day first, so 04-11-2026 becomes 2026-11-04.
	- Write time as HH:MM:SS, or an empty string when the email has no time.

	## Output shapes
	Use when the email represents a completed debit or credit.

	{
	  "account": "HDFC 1234",
	  "merchant": "Google Play",
	  "currency": "Rs",
	  "amount": -650.00,
	  "date": "2026-06-14",
	  "time": "10:00:00"
	}

	EXAMPLES: 
  Input: "Dear Customer, Greetings from ABCD Bank! Rs.500.00 has been debited from account 4567 to VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 14-07-25. Your UPI transaction reference number is 123456789012. If you did not authorize this transaction, please report it immediately by calling 18002586161 Or SMS BLOCK UPI to 9000000205. Warm Regards, HDFC BankFor more details on Service charges and Fees, click here.. © HDFC Bank"
	Output: {"account": "ABCD 4567", "merchant": "JOHN DOE S O JAMES DOE", "currency": "Rs", "amount": -500.0, "date": "2025-07-14", "time": ""}

	Input: "Dear Customer, Rs. 3000.00 is successfully credited to your account **4567 by VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 12-07-25. Your UPI transaction reference number is 987654321098. Thank you for banking with us. Warm Regards, HDFC BankFor more details on Service charges and Fees, click here.. © HDFC Bank"
	Output: {"account": "HDFC 4567", "merchant": "JOHN DOE S O JAMES DOE", "currency": "Rs", "amount": 3000.0, "date": "2025-07-12", "time": ""}

	Input: "Dear Customer, Rs.1200.00 has been debited from your HDFC Bank Credit Card ending 9876 towards NETFLIX on 05 Jan, 2025 at 10:22:11."
	Output: {"account": "HDFC 9876", "merchant": "NETFLIX", "currency": "Rs", "amount": -1200.0, "date": "2025-01-05", "time": "10:22:11"}

	Input: "Dear Customer, Thank you for using your HDFC Bank Card XX1234 for Rs. 13.04 at GOOGLE CLOUD CYBS SI on 04-11-2026 00:19:43. Authorization code: 045383 Please note that this transaction was conducted without OTP/PIN. Important Note: If you did not authorise this transaction, please act immediately. Your safety is our top priority, and we are here to help."
	Output: {"account": "HDFC 1234", "merchant": "GOOGLE CLOUD CYBS SI", "currency": "Rs", "amount": -13.04, "date": "2026-11-04", "time": "00:19:43"}

	Input: "Dear Customer, Greetings from HDFC Bank! A transaction reversal of Rs. 250.00 has been initiated to your HDFC Bank Credit Card ending 9876 From Merchant: NETFLIX* AB12CD34EF Date Time: 12 Mar, 2026 at 09:15:30 Please allow up to 48 hours for the reversal to reflect in your card statement."
	Output: {"account": "HDFC 9876", "merchant": "NETFLIX", "currency": "Rs", "amount": 250.0, "date": "2026-03-12", "time": "09:15:30"}

	Input: "Dear Customer, Rs.4500.00 has been debited from HDFC Bank Account Number XXXXXXXXXX4567 towards ACME Finance Ltd/123456789 with UMRN ABCD1234567890 on 03-Feb-2026."
	Output: {"account": "HDFC 4567", "merchant": "ACME Finance Ltd", "currency": "Rs", "amount": -4500.0, "date": "2026-02-03", "time": ""}

	Input: "Dear Customer, Greetings from HDFC Bank! Your Canva Pty Ltd bill, set up through E-mandate (Auto payment), has been successfully paid using your HDFC Bank Credit Card ending 1234. Transaction Details: Amount: INR 500.00 Date: 10/06/2026 SI Hub ID: Y8Inwhnbjn To manage your e-Mandates, please visit: https://www.sihub.in/managesi/hdfcbank Thank you for banking with us. Warm regards, HDFC BankFor more details on Service charges and Fees, click here.. © HDFC Bank"
	Output: {"account": "", "merchant": "", "currency": "", "amount": 0.0, "date": "", "time": ""}

	The next message is the input to process.`

// checkTransactionPrompt asks a chat model for a ports.TxnVerdict.
const checkTransactionPrompt = `You decide whether a message from a bank or card issuer reports a completed transaction. Output strictly JSON. Do not wrap the response in markdown blocks.

{
  "messageType": "transaction" or "statement",
  "isTransaction": true or false,
  "type": "debit", "credit" or "none",
  "confidence": a number between 0 and 1
}

isTransaction is true only for an alert about a completed debit, credit, purchase, transfer or refund. It is false for newsletters, merchant receipts, account or card notices, OTPs, upcoming payment notices and e-mandate or auto-payment confirmations.
type is "debit" when money moved out, "credit" when money moved in, and "none" when no completed movement is reported.`

// predictionPrompt asks a chat model for a payee and category. {categories} is
// replaced with the budget's category names.
const predictionPrompt = `You are an expert financial data extraction API. Your job is to analyze raw bank transaction text and output strictly valid JSON.

Extract the clean merchant brand name and categorize the transaction into exactly ONE of the allowed categories.

RULES:
1. MERCHANT NAME: Extract the core brand. Remove all bank jargon (UPI, POS, REF, VPA), dates, and reference numbers. (e.g., "PYU*Swiggy Food 12-Apr" -> "Swiggy").
2. CATEGORY: You must select exactly one category from the ALLOWED CATEGORIES list. If you are completely unsure, use an empty string.
3. JSON ONLY: Do not wrap the response in markdown blocks. Return only the raw JSON object.
4. Never output the data from the examples. Only process the provided input.

ALLOWED CATEGORIES:
{categories}

EXPECTED JSON SCHEMA:
{
  "merchantName": "string",
  "suggestedTag": "string",
  "confidence": integer (0-100, how sure you are of the category),
  "payeeConfidence": integer (0-100, how sure you are of the merchant name),
  "reasoning": "string (Brief 1-sentence explanation of why you chose this category)"
}`
