// Package model holds cipher's own types that more than one package reads.
package model

// LLMPrediction is the JSON a prediction prompt asks the model for: the old
// email flow's (service) and the job pipeline's (adapters/llm).
type LLMPrediction struct {
	MerchantName string `json:"merchantName"`
	SuggestedTag string `json:"suggestedTag"`
	Confidence   int32  `json:"confidence"` // in the category
	// PayeeConfidence is nil from models (and prompts) that don't give it.
	PayeeConfidence *int32 `json:"payeeConfidence"`
	Reasoning       string `json:"reasoning"`
}
