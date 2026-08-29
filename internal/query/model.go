package query

type Terms struct {
	Words       []string `json:"words"`
	Identifiers []string `json:"identifiers"`
	Phrases     []string `json:"phrases"`
	Paths       []string `json:"paths"`
	Symbols     []string `json:"symbols"`
	UnicodeRuns []string `json:"unicode_runs,omitempty"`
}

type Intent string

const (
	IntentDefinition Intent = "definition"
	IntentCallers    Intent = "callers"
	IntentCallees    Intent = "callees"
	IntentTests      Intent = "tests"
	IntentImports    Intent = "imports"
	IntentExports    Intent = "exports"
	IntentReferences Intent = "references"
	IntentImpact     Intent = "impact"
)

type Plan struct {
	RawQuery      string   `json:"raw_query"`
	Terms         Terms    `json:"terms"`
	Intents       []Intent `json:"intents"`
	PrimaryIntent Intent   `json:"primary_intent"`
	Anchors       []string `json:"anchors"`
	Relations     []string `json:"relations"`
	Profile       string   `json:"profile"`
}

func (p Plan) HasIntent(intent Intent) bool {
	for _, candidate := range p.Intents {
		if candidate == intent {
			return true
		}
	}
	return false
}

func (p Plan) IntentStrings() []string {
	result := make([]string, len(p.Intents))
	for index, intent := range p.Intents {
		result[index] = string(intent)
	}
	return result
}
