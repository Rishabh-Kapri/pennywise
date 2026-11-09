package decisionsAdapter

import "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"

type state struct {
	Query string `json:"query"`
}

type intentType string

const (
	choiceIntentType intentType = "choice"
	noulIntentType   intentType = "noul"
)

type intent struct {
	Type         intentType         `json:"type"`
	Instructions string             `json:"instructions"`
	Criteria     map[string]*string `json:"criteria"`
}

type decisionQuery struct {
	State     state             `json:"state"`
	Model     string            `json:"model"`
	Questions map[string]intent `json:"questions"`
}

type answer struct {
	Type          intentType         `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        *string            `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Box           *boxCoords         `json:"box,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Legend        map[string]any     `json:"legend,omitempty"`
}

type boxCoords struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bootom"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
}

type usage struct {
	InputTokens  int      `json:"input_tokens"`
	OutputTokens int      `json:"output_tokens"`
	Cost         *float64 `json:"cost"`
}

type decisionResponse struct {
	Model    string            `json:"model"`
	Answers  map[string]answer `json:"answers"`
	Usage    usage             `json:"usage"`
	ID       *string           `json:"id"`
	Provider *string           `json:"provider"`
}

func (r *decisionResponse) usage() *ports.Usage {
	u := &ports.Usage{
		Model:        r.Model,
		InputTokens:  r.Usage.InputTokens,
		OutputTokens: r.Usage.OutputTokens,
		Cost:         r.Usage.Cost,
	}
	if r.Provider != nil {
		u.Provider = *r.Provider
	}
	return u
}
