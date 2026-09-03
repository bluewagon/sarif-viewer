package backend

type FindingKey struct {
	RunIndex    int `json:"runIndex"`
	ResultIndex int `json:"resultIndex"`
}

type LocationDTO struct {
	URI         string `json:"uri"`
	StartLine   int    `json:"startLine"`
	StartColumn int    `json:"startColumn"`
	EndLine     int    `json:"endLine"`
	EndColumn   int    `json:"endColumn"`
	Snippet     string `json:"snippet"`
}

type FindingDTO struct {
	Key             FindingKey  `json:"key"`
	RunName         string      `json:"runName"`
	ToolName        string      `json:"toolName"`
	RuleID          string      `json:"ruleId"`
	RuleName        string      `json:"ruleName"`
	RuleDescription string      `json:"ruleDescription"`
	HelpURI         string      `json:"helpUri"`
	Message         string      `json:"message"`
	SARIFLevel      string      `json:"sarifLevel"`
	Severity        string      `json:"severity"`
	Disposition     string      `json:"disposition"`
	Comment         string      `json:"comment"`
	ReviewedAt      string      `json:"reviewedAt"`
	Location        LocationDTO `json:"location"`
}

type RunSummaryDTO struct {
	Index        int    `json:"index"`
	Name         string `json:"name"`
	ToolName     string `json:"toolName"`
	FindingCount int    `json:"findingCount"`
}

type SARIFDocumentDTO struct {
	DocumentID   string          `json:"documentId"`
	FileName     string          `json:"fileName"`
	SourcePath   string          `json:"sourcePath"`
	Version      string          `json:"version"`
	FindingCount int             `json:"findingCount"`
	Runs         []RunSummaryDTO `json:"runs"`
	Findings     []FindingDTO    `json:"findings"`
}

type FindingReview struct {
	RunIndex    int    `json:"runIndex"`
	ResultIndex int    `json:"resultIndex"`
	Severity    string `json:"severity"`
	Disposition string `json:"disposition"`
	Comment     string `json:"comment"`
	ReviewedAt  string `json:"reviewedAt"`
}

type loadedSARIF struct {
	document   map[string]any
	sourcePath string
	dto        SARIFDocumentDTO
}
