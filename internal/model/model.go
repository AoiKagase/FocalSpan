package model

type SourceFile struct {
	Path      string
	Language  string
	Content   []byte
	SHA256    string
	SizeBytes int64
}

type Symbol struct {
	Handle        string
	FilePath      string
	Language      string
	Kind          string
	Name          string
	QualifiedName string
	Signature     string
	StartLine     int
	EndLine       int
	StartByte     int
	EndByte       int
	ParentHandle  string
	Confidence    float64
}

type Chunk struct {
	Handle          string
	FilePath        string
	Language        string
	Kind            string
	SymbolHandle    string
	SymbolName      string
	Signature       string
	StartLine       int
	EndLine         int
	StartByte       int
	EndByte         int
	Content         string
	ContentHash     string
	EstimatedTokens int
}

type Relation struct {
	FromHandle   string
	ToHandle     string
	UnresolvedTo string
	Kind         string
	Confidence   float64
	Source       string
}

type Diagnostic struct {
	FilePath string
	Level    string
	Code     string
	Message  string
}

type Extraction struct {
	Symbols     []Symbol
	Chunks      []Chunk
	Relations   []Relation
	Diagnostics []Diagnostic
}

type ScoreReason struct {
	Code   string  `json:"code"`
	Weight float64 `json:"weight"`
	Detail string  `json:"detail"`
}

type RankedCandidate struct {
	Handle          string
	Path            string
	Language        string
	Kind            string
	Symbol          string
	Signature       string
	StartLine       int
	EndLine         int
	StartByte       int
	EndByte         int
	Content         string
	ContentHash     string
	Score           float64
	RetrievalScore  float64
	Confidence      float64
	Reasons         []ScoreReason
	EstimatedTokens int
	Changed         bool
	Relation        string
}

type PackRequest struct {
	Query         string
	IndexRevision string
	TokenBudget   int
	Mode          string
	Candidates    []RankedCandidate
	IntentHints   []string
}

type TokenSavings struct {
	BaselineTokens int     `json:"baseline_tokens"`
	SavedTokens    int     `json:"saved_tokens"`
	SavingsRatio   float64 `json:"savings_ratio"`
}

type ContextBundle struct {
	Query           string        `json:"query"`
	IndexRevision   string        `json:"index_revision"`
	BudgetTokens    int           `json:"budget_tokens"`
	EstimatedTokens int           `json:"estimated_tokens"`
	Savings         *TokenSavings `json:"token_savings,omitempty"`
	Truncated       bool          `json:"truncated"`
	Items           []ContextItem `json:"items"`
	OmittedCount    int           `json:"omitted_count"`
	Diagnostics     []string      `json:"diagnostics,omitempty"`
}

type ContextItem struct {
	Handle          string        `json:"handle"`
	Path            string        `json:"path"`
	Language        string        `json:"language"`
	Kind            string        `json:"kind"`
	Symbol          string        `json:"symbol"`
	Signature       string        `json:"signature,omitempty"`
	StartLine       int           `json:"start_line"`
	EndLine         int           `json:"end_line"`
	Score           float64       `json:"score"`
	Reasons         []ScoreReason `json:"reasons,omitempty"`
	Relation        string        `json:"relation,omitempty"`
	EstimatedTokens int           `json:"estimated_tokens"`
	Content         string        `json:"content,omitempty"`
	Elided          bool          `json:"elided,omitempty"`
}

type IndexRun struct {
	ID             int64  `json:"id"`
	StartedAt      string `json:"started_at"`
	CompletedAt    string `json:"completed_at"`
	FilesSeen      int    `json:"files_seen"`
	FilesAdded     int    `json:"files_added"`
	FilesChanged   int    `json:"files_changed"`
	FilesUnchanged int    `json:"files_unchanged"`
	FilesDeleted   int    `json:"files_deleted"`
	ParseFailures  int    `json:"parse_failures"`
	DurationMS     int64  `json:"duration_ms"`
	Revision       string `json:"revision"`
}

type Status struct {
	Root            string `json:"root"`
	DBPath          string `json:"db_path"`
	SchemaVersion   string `json:"schema_version"`
	LastRevision    string `json:"last_revision"`
	FileCount       int    `json:"file_count"`
	SymbolCount     int    `json:"symbol_count"`
	ChunkCount      int    `json:"chunk_count"`
	RelationCount   int    `json:"relation_count"`
	DiagnosticCount int    `json:"diagnostic_count"`
	Stale           bool   `json:"stale"`
	LastDurationMS  int64  `json:"last_index_duration_ms"`
}

type HealthStatus struct {
	Status
	RepositoryDetected bool     `json:"repository_detected"`
	ConfigValid        bool     `json:"config_valid"`
	DBOpen             bool     `json:"db_open"`
	FTS5               bool     `json:"fts5"`
	PathPermissions    bool     `json:"path_permissions"`
	MCPReady           bool     `json:"mcp_ready"`
	IndexFresh         bool     `json:"index_fresh"`
	Ready              bool     `json:"ready"`
	ConfigWarnings     []string `json:"config_warnings,omitempty"`
	Diagnostics        []string `json:"diagnostics,omitempty"`
}
