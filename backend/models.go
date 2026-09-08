package backend

type FindingKey struct {
	RunIndex    int `json:"runIndex"`
	ResultIndex int `json:"resultIndex"`
}

type LocationDTO struct {
	URI              string `json:"uri"`
	StartLine        int    `json:"startLine"`
	StartColumn      int    `json:"startColumn"`
	EndLine          int    `json:"endLine"`
	EndColumn        int    `json:"endColumn"`
	Snippet          string `json:"snippet"`
	SnippetStartLine int    `json:"snippetStartLine"`
	SnippetOrigin    string `json:"snippetOrigin"`
	SnippetStatus    string `json:"snippetStatus"`
}

type SourceSelectionDTO struct {
	Kind              string               `json:"kind"`
	Location          string               `json:"location"`
	ContextLines      int                  `json:"contextLines"`
	GitAuthentication GitAuthenticationDTO `json:"gitAuthentication"`
}

type GitAuthenticationDTO struct {
	Provider            string `json:"provider"`
	Username            string `json:"username"`
	PersonalAccessToken string `json:"personalAccessToken"`
}

type SnippetSummaryDTO struct {
	Generated   int `json:"generated"`
	Embedded    int `json:"embedded"`
	Unavailable int `json:"unavailable"`
}

type AffectedRouteDTO struct {
	Route  string `json:"route"`
	Method string `json:"method"`
}

type CodeFlowStepDTO struct {
	Message        string      `json:"message"`
	Location       LocationDTO `json:"location"`
	ExecutionOrder int         `json:"executionOrder"`
	NestingLevel   int         `json:"nestingLevel"`
}

type ThreadFlowDTO struct {
	ID      string            `json:"id"`
	Message string            `json:"message"`
	Steps   []CodeFlowStepDTO `json:"steps"`
}

type CodeFlowDTO struct {
	Message     string          `json:"message"`
	ThreadFlows []ThreadFlowDTO `json:"threadFlows"`
}

type FindingDTO struct {
	Key                   FindingKey       `json:"key"`
	RunName               string           `json:"runName"`
	ToolName              string           `json:"toolName"`
	RuleID                string           `json:"ruleId"`
	RuleName              string           `json:"ruleName"`
	RuleDescription       string           `json:"ruleDescription"`
	HelpURI               string           `json:"helpUri"`
	Message               string           `json:"message"`
	SARIFLevel            string           `json:"sarifLevel"`
	Severity              string           `json:"severity"`
	Disposition           string           `json:"disposition"`
	Comment               string           `json:"comment"`
	ReviewedAt            string           `json:"reviewedAt"`
	Location              LocationDTO      `json:"location"`
	FindingID             string           `json:"findingId"`
	IsReachable           *bool            `json:"isReachable"`
	VulnerabilityEvidence string           `json:"vulnerabilityEvidence"`
	AffectedRoute         AffectedRouteDTO `json:"affectedRoute"`
	CodeFlows             []CodeFlowDTO    `json:"codeFlows"`
}

type RunSummaryDTO struct {
	Index        int    `json:"index"`
	Name         string `json:"name"`
	ToolName     string `json:"toolName"`
	FindingCount int    `json:"findingCount"`
}

type SARIFDocumentDTO struct {
	DocumentID     string            `json:"documentId"`
	FileName       string            `json:"fileName"`
	SourcePath     string            `json:"sourcePath"`
	Version        string            `json:"version"`
	FindingCount   int               `json:"findingCount"`
	Runs           []RunSummaryDTO   `json:"runs"`
	Findings       []FindingDTO      `json:"findings"`
	SnippetSummary SnippetSummaryDTO `json:"snippetSummary"`
}

type FindingReview struct {
	RunIndex    int    `json:"runIndex"`
	ResultIndex int    `json:"resultIndex"`
	Severity    string `json:"severity"`
	Disposition string `json:"disposition"`
	Comment     string `json:"comment"`
	Reviewer    string `json:"reviewer"`
	ReviewedAt  string `json:"reviewedAt"`
}

type loadedSARIF struct {
	document    map[string]any
	sourcePath  string
	dto         SARIFDocumentDTO
	cleanupPath string
}
