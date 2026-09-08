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
			_, err := NewSARIFService().LoadSARIF(path, SourceSelectionDTO{Kind: "none", ContextLines: 3})
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
        "properties": {"threathound/reviewStatus": {
          "severity": "low",
          "status": "confirmed",
          "rationale": "Expected review",
          "reviewedAt": "2026-09-02T12:30:00Z"
        }}
      }]
    }
  ]
}`)

	document, err := NewSARIFService().LoadSARIF(path, SourceSelectionDTO{Kind: "none", ContextLines: 3})
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

func TestLoadSARIFNormalizesThreatHoundMetadataAndCodeFlows(t *testing.T) {
	path := writeTestFile(t, "threathound.sarif", `{
  "version": "2.1.0",
  "runs": [{
    "originalUriBaseIds": {"ROOT": {"uri": "file:///repo/"}},
    "tool": {"driver": {"name": "ThreatHound"}},
    "threadFlowLocations": [{
      "location": {
        "message": {"text": "Value reaches the cached call"},
        "physicalLocation": {
          "artifactLocation": {"uri": "src/cached.go", "uriBaseId": "ROOT"},
          "region": {"startLine": 24, "startColumn": 7}
        }
      },
      "executionOrder": 99,
      "nestingLevel": 1
    }],
    "results": [{
      "ruleId": "TH001",
      "message": {"text": "Untrusted input reaches a sink"},
      "fingerprints": {"threathound/findingId/v1": "finding-123"},
      "properties": {
        "threathound/isReachable": {"reachable": false},
        "threathound/vulnerabilityEvidence": "Input is copied without validation.\nThe sink executes on every request.",
        "threathound/affectedRoute": {"route": "/api/orders/:id", "method": "post"}
      },
      "codeFlows": [{
        "message": {"text": "Request data reaches the command runner"},
        "threadFlows": [{
          "id": "request-thread",
          "message": {"markdown": "Request processing"},
          "locations": [{
            "location": {
              "message": {"text": "Read the route parameter"},
              "physicalLocation": {
                "artifactLocation": {"uri": "src/routes.go", "uriBaseId": "ROOT"},
                "region": {"startLine": 10, "startColumn": 3, "endLine": 10, "endColumn": 18}
              }
            },
            "executionOrder": 1
          }, {
            "index": 0,
            "executionOrder": 2,
            "nestingLevel": 2
          }, {
            "location": {"message": {"text": "Invoke the vulnerable sink"}},
            "executionOrder": 3
          }]
        }, {
          "id": "audit-thread",
          "locations": [{"location": {"message": {"text": "Record the request"}}}]
        }]
      }, {
        "threadFlows": [{"message": {"text": "Alternate flow"}, "locations": []}]
      }]
    }, {
      "ruleId": "TH002",
      "message": {"text": "Optional metadata is malformed"},
      "fingerprints": {"threathound/findingId/v1": false},
      "properties": {
        "threathound/isReachable": {"reachable": "false"},
        "threathound/vulnerabilityEvidence": {},
        "threathound/affectedRoute": "not-an-object"
      },
      "codeFlows": {"threadFlows": []}
    }]
  }]
}`)

	document, err := NewSARIFService().LoadSARIF(path, SourceSelectionDTO{Kind: "none", ContextLines: 3})
	if err != nil {
		t.Fatalf("LoadSARIF() error = %v", err)
	}
	finding := document.Findings[0]
	if finding.FindingID != "finding-123" || finding.IsReachable == nil || *finding.IsReachable {
		t.Fatalf("unexpected finding identity/reachability: %+v", finding)
	}
	if finding.VulnerabilityEvidence != "Input is copied without validation.\nThe sink executes on every request." {
		t.Fatalf("unexpected evidence: %q", finding.VulnerabilityEvidence)
	}
	if finding.AffectedRoute.Method != "post" || finding.AffectedRoute.Route != "/api/orders/:id" {
		t.Fatalf("unexpected route: %+v", finding.AffectedRoute)
	}
	if len(finding.CodeFlows) != 2 || finding.CodeFlows[0].Message != "Request data reaches the command runner" {
		t.Fatalf("unexpected code flows: %+v", finding.CodeFlows)
	}
	threads := finding.CodeFlows[0].ThreadFlows
	if len(threads) != 2 || threads[0].ID != "request-thread" || threads[0].Message != "Request processing" {
		t.Fatalf("unexpected thread flows: %+v", threads)
	}
	steps := threads[0].Steps
	if len(steps) != 3 || steps[0].Message != "Read the route parameter" || steps[0].Location.URI != "file:///repo/src/routes.go" {
		t.Fatalf("unexpected direct flow step: %+v", steps)
	}
	if steps[1].Message != "Value reaches the cached call" || steps[1].Location.URI != "file:///repo/src/cached.go" || steps[1].ExecutionOrder != 2 || steps[1].NestingLevel != 2 {
		t.Fatalf("cached flow step was not resolved and overridden: %+v", steps[1])
	}
	if steps[2].Message != "Invoke the vulnerable sink" || steps[2].Location.URI != "" || steps[2].ExecutionOrder != 3 {
		t.Fatalf("message-only flow step was not retained: %+v", steps[2])
	}
	if threads[1].Steps[0].ExecutionOrder != -1 {
		t.Fatalf("missing execution order = %d, want -1", threads[1].Steps[0].ExecutionOrder)
	}

	malformed := document.Findings[1]
	if malformed.FindingID != "" || malformed.IsReachable != nil || malformed.VulnerabilityEvidence != "" || malformed.AffectedRoute != (AffectedRouteDTO{}) || len(malformed.CodeFlows) != 0 {
		t.Fatalf("malformed optional metadata was not ignored: %+v", malformed)
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
