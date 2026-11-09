package model

import "encoding/json"

type State struct {
	Query string `json:"query"`
}

type IntentType string

const (
	ChoiceIntentType IntentType = "choice"
	NoulIntentType   IntentType = "noul"
	ScoreIntentType  IntentType = "score"
)

type Intent struct {
	Type         IntentType         `json:"type"`
	Instructions string             `json:"instructions,omitempty"`
	Criteria     map[string]*string `json:"criteria"`
}

type Question struct {
	Type         IntentType
	Instructions string
	Criteria     map[string]string // noul: "truel/"false"; choice: label -> guidance
}

type AnswerType string

type Answer struct {
	Type          AnswerType                 `json:"type"` // noul, choice, score, refusal
	Noul          *float64                   `json:"noul,omitempty"`
	Choice        *string                    `json:"choice,omitempty"`
	Score         *float64                   `json:"score,omitempty"`
	Box           *BoxCoords                 `json:"box,omitempty"`
	Confidence    *float64                   `json:"confidence,omitempty"`
	Probabilities map[string]float64         `json:"probabilities,omitempty"` // not for noul
	Legend        map[string]json.RawMessage `json:"legend,omitempty"`
}

type BoxCoords struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
}

type DecisionRequest struct {
	State     State             `json:"state"`
	Model     string            `json:"model"`
	Questions map[string]Intent `json:"questions"`
}

type DecisionResponse struct {
	Model    string            `json:"model"`
	Answers  map[string]Answer `json:"answers"`
	Usage    Usage             `json:"usage"`
	ID       *string           `json:"id"`
	Provider *string           `json:"provider"`
}
