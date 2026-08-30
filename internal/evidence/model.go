package evidence

import (
	"encoding/json"
	"fmt"
)

const SchemaContextV1 = "focalspan.context.v1"

type Mode string

const (
	ModeOutline Mode = "outline"
	ModeFocused Mode = "focused"
	ModeSource  Mode = "source"
)

type Role string

const (
	RoleTarget         Role = "target"
	RoleDefinition     Role = "definition"
	RoleDeclaration    Role = "declaration"
	RoleImplementation Role = "implementation"
	RoleCaller         Role = "caller"
	RoleCallee         Role = "callee"
	RoleTest           Role = "test"
	RoleType           Role = "type"
	RoleImport         Role = "import"
	RoleExport         Role = "export"
	RoleReference      Role = "reference"
	RoleConfig         Role = "config"
	RoleTemplate       Role = "template"
	RoleDocumentation  Role = "documentation"
	RoleChange         Role = "change"
	RoleDependent      Role = "dependent"
	RoleContext        Role = "context"
)

type Fidelity string

const (
	FidelityVerbatim  Fidelity = "verbatim"
	FidelityExcerpt   Fidelity = "excerpt"
	FidelitySignature Fidelity = "signature"
	FidelitySynthetic Fidelity = "synthetic"
)

type Certainty string

const (
	CertaintyExact   Certainty = "exact"
	CertaintyScoped  Certainty = "scoped"
	CertaintyLexical Certainty = "lexical"
)

const (
	SegmentSource  = "source"
	SegmentOmitted = "omitted"
)

type Location struct {
	Path  string `json:"path"`
	Lines [2]int `json:"lines"`
}

type Segment struct {
	Kind  string `json:"kind"`
	Lines [2]int `json:"lines"`
	Text  string `json:"text,omitempty"`
}

type Item struct {
	ID        string    `json:"id"`
	Handle    string    `json:"handle"`
	Role      Role      `json:"role"`
	Location  Location  `json:"location"`
	Language  string    `json:"language,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	Symbol    string    `json:"symbol,omitempty"`
	Signature string    `json:"signature,omitempty"`
	Fidelity  Fidelity  `json:"fidelity"`
	Why       []string  `json:"why,omitempty"`
	Source    string    `json:"source,omitempty"`
	Segments  []Segment `json:"segments,omitempty"`
	Outline   string    `json:"outline,omitempty"`
}

type Edge struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Kind      string    `json:"kind"`
	Certainty Certainty `json:"certainty"`
}

type Budget struct {
	Limit     int  `json:"limit"`
	Used      int  `json:"used"`
	Truncated bool `json:"truncated,omitempty"`
	Omitted   int  `json:"omitted,omitempty"`
}

type NextAction struct {
	Handle   string `json:"handle"`
	Relation string `json:"relation"`
	Reason   string `json:"reason"`
}

type Packet struct {
	Schema       string       `json:"schema"`
	Revision     string       `json:"revision,omitempty"`
	Intent       string       `json:"intent,omitempty"`
	Mode         Mode         `json:"mode"`
	Budget       Budget       `json:"budget"`
	Evidence     []Item       `json:"evidence"`
	Relations    []Edge       `json:"relations,omitempty"`
	Limitations  []string     `json:"limitations,omitempty"`
	Next         []NextAction `json:"next,omitempty"`
	SkippedKnown int          `json:"skipped_known,omitempty"`
}

func (p Packet) MarshalJSON() ([]byte, error) {
	type packetAlias Packet
	copy := packetAlias(p)
	if copy.Evidence == nil {
		copy.Evidence = []Item{}
	}
	return json.Marshal(copy)
}

func AssignLocalIDs(items []Item) map[string]string {
	ids := make(map[string]string, len(items))
	for i := range items {
		items[i].ID = fmt.Sprintf("e%d", i+1)
		if items[i].Handle != "" {
			if _, exists := ids[items[i].Handle]; !exists {
				ids[items[i].Handle] = items[i].ID
			}
		}
	}
	return ids
}
