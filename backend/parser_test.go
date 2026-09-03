package backend

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLoadSARIFValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "malformed JSON", content: `{`, want: "parse SARIF file"},
		{name: "wrong root", content: `[]`, want: "root must be a JSON object"},
		{name: "missing version", content: `{"runs":[]}`, want: "version is required"},
		{name: "unsupported version", content: `{"version":"2.0.0","runs":[{}]}`, want: "unsupported SARIF version"},
		{name: "missing runs", content: `{"version":"2.1.0"}`, want: "runs must be a non-empty array"},
		{name: "invalid results", content: `{"version":"2.1.0","runs":[{"results":{}}]}`, want: "results for run 1 must be an array"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeTestFile(t, "input.sarif", test.content)
			_, err := NewSARIFService().LoadSARIF(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadSARIF() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestLoadSARIFNormalizesRunsRulesMessagesAndLocations(t *testing.T) {
	path := writeTestFile(t, "scan.sarif", `{
  "version": "2.1.0",
  "original": 900719925474099312345,
  "runs": [
    {
      "automationDetails": {"id": "nightly/build-12"},
      "originalUriBaseIds": {"ROOT": {"uri": "file:///repo/"}},
      "tool": {"driver": {
        "name": "Scanner",
        "rules": [{
          "id": "SEC001",
          "name": "UnsafeCall",
          "shortDescription": {"text": "Unsafe function call"},
          "helpUri": "https://example.test/rules/SEC001",
          "properties": {"security-severity": "9.4"},
          "messageStrings": {"default": {"text": "Call to {0} is unsafe"}}
        }]
      }},
      "results": [{
        "ruleId": "SEC001",
        "ruleIndex": 0,
        "message": {"id": "default", "arguments": ["eval"]},
        "locations": [{"physicalLocation": {
          "artifactLocation": {"uri": "src/app.ts", "uriBaseId": "ROOT"},
          "region": {"startLine": 7, "startColumn": 3, "endLine": 7, "endColumn": 9,
            "snippet": {"text": "eval(input)"}}
        }}]
      }]
    },
    {
      "tool": {"driver": {"name": "Second"}},
      "results": [{
        "ruleId": "INFO",
        "level": "none",
        "message": {"markdown": "Informational result"},
        "properties": {"sastafras": {
          "severity": "low",
          "disposition": "confirmed",
          "comment": "Expected review",
          "reviewedAt": "2026-09-02T12:30:00Z"
        }}
      }]
    }
  ]
}`)

	document, err := NewSARIFService().LoadSARIF(path)
	if err != nil {
		t.Fatalf("LoadSARIF() error = %v", err)
	}
	if document.FindingCount != 2 || len(document.Runs) != 2 {
		t.Fatalf("counts = %d findings, %d runs", document.FindingCount, len(document.Runs))
	}
	first := document.Findings[0]
	if first.RunName != "nightly/build-12" || first.ToolName != "Scanner" || first.RuleName != "UnsafeCall" {
		t.Fatalf("unexpected first finding identity: %+v", first)
	}
	if first.Message != "Call to eval is unsafe" || first.Severity != "critical" {
		t.Fatalf("unexpected message/severity: %+v", first)
	}
	if first.Location.URI != "file:///repo/src/app.ts" || first.Location.StartLine != 7 || first.Location.Snippet != "eval(input)" {
		t.Fatalf("unexpected location: %+v", first.Location)
	}
	second := document.Findings[1]
	if second.Severity != "low" || second.Disposition != "confirmed" || second.Comment != "Expected review" {
		t.Fatalf("existing review metadata not loaded: %+v", second)
	}
}

func TestSeverityInference(t *testing.T) {
	tests := []struct {
		value any
		want  string
	}{
		{json.Number("9.0"), "critical"},
		{"7.0", "high"},
		{json.Number("4"), "medium"},
		{json.Number("0.1"), "low"},
		{json.Number("0"), "informational"},
		{"not-a-number", ""},
	}
	for _, test := range tests {
		if got := severityFromScore(test.value); got != test.want {
			t.Errorf("severityFromScore(%v) = %q, want %q", test.value, got, test.want)
		}
	}
	levels := map[string]string{"error": "high", "warning": "medium", "note": "low", "none": "informational"}
	for level, want := range levels {
		if got := severityFromLevel(level); got != want {
			t.Errorf("severityFromLevel(%q) = %q, want %q", level, got, want)
		}
	}
}
