package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportSARIFMergesReviewsAndPreservesUnknownContent(t *testing.T) {
	service := NewSARIFService()
	source := writeTestFile(t, "source.sarif", `{
  "version":"2.1.0",
  "unknownRoot":{"large":900719925474099312345},
  "runs":[{"tool":{"driver":{"name":"Tool"}},"results":[{
    "ruleId":"R1","level":"warning","message":{"text":"Finding"},
    "fingerprints":{"threathound/findingId/v1":"finding-preserved"},
    "codeFlows":[{"threadFlows":[{"locations":[{"location":{"message":{"text":"Preserve this step"}}}]}]}],
    "unknownResult":[1,2,3],
    "properties":{"owner":"security","threathound/isReachable":{"reachable":false},"sarif-viewer":{"custom":"keep"}}
  }]}]
}`)
	document, err := service.LoadSARIF(source, SourceSelectionDTO{Kind: "none", ContextLines: 3})
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "reviewed.sarif")
	review := FindingReview{
		RunIndex: 0, ResultIndex: 0, Severity: "critical", Disposition: "false-positive",
		Comment: "  Accepted test fixture  ", Reviewer: "  security-reviewer  ", ReviewedAt: "2026-09-02T12:30:00Z",
	}
	if err := service.ExportSARIF(document.DocumentID, destination, []FindingReview{review}); err != nil {
		t.Fatalf("ExportSARIF() error = %v", err)
	}
	exportedData, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exportedData), "900719925474099312345") {
		t.Fatalf("large unknown number was not preserved: %s", exportedData)
	}
	exported, err := decodeJSONObject(exportedData)
	if err != nil {
		t.Fatal(err)
	}
	result, err := resultAt(exported, FindingKey{RunIndex: 0, ResultIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if firstString(result["level"]) != "error" || nestedValue(result, "properties", "owner") != "security" {
		t.Fatalf("standard or existing properties were not retained: %+v", result)
	}
	if nestedValue(result, "fingerprints", "threathound/findingId/v1") != "finding-preserved" || nestedValue(result, "properties", "threathound/isReachable", "reachable") != false {
		t.Fatalf("ThreatHound metadata was not preserved: %+v", result)
	}
	codeFlows, _ := arrayValue(result["codeFlows"])
	if len(codeFlows) != 1 || nestedValue(codeFlows[0], "threadFlows") == nil {
		t.Fatalf("code flows were not preserved: %+v", result["codeFlows"])
	}
	legacyMetadata, _ := objectValue(nestedValue(result, "properties", "sarif-viewer"))
	if legacyMetadata["custom"] != "keep" {
		t.Fatalf("existing application metadata was not preserved: %+v", legacyMetadata)
	}
	metadata, _ := objectValue(nestedValue(result, "properties", "threathound/reviewStatus"))
	if metadata["rationale"] != "Accepted test fixture" || metadata["reviewer"] != "security-reviewer" || metadata["status"] != "false-positive" || metadata["reviewed_at"] != "2026-09-02T12:30:00Z" {
		t.Fatalf("review metadata was not merged: %+v", metadata)
	}
	if _, exists := metadata["comment"]; exists {
		t.Fatalf("legacy comment key was written: %+v", metadata)
	}
	if _, exists := metadata["disposition"]; exists {
		t.Fatalf("legacy disposition key was written: %+v", metadata)
	}
	if _, exists := metadata["reviewedAt"]; exists {
		t.Fatalf("legacy reviewedAt key was written: %+v", metadata)
	}
}

func TestExportSARIFValidation(t *testing.T) {
	service := NewSARIFService()
	source := writeTestFile(t, "source.sarif", `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"Tool"}},"results":[{"message":{"text":"Finding"}}]}]}`)
	document, err := service.LoadSARIF(source, SourceSelectionDTO{Kind: "none", ContextLines: 3})
	if err != nil {
		t.Fatal(err)
	}
	valid := FindingReview{RunIndex: 0, ResultIndex: 0, Severity: "high", Disposition: "confirmed", Comment: "Reviewed", Reviewer: "reviewer", ReviewedAt: "2026-09-02T12:30:00Z"}
	tests := []struct {
		name        string
		documentID  string
		destination string
		reviews     []FindingReview
		want        string
	}{
		{name: "stale document", documentID: "wrong", destination: filepath.Join(t.TempDir(), "out.sarif"), reviews: []FindingReview{valid}, want: "document has changed"},
		{name: "same path", documentID: document.DocumentID, destination: source, reviews: []FindingReview{valid}, want: "different file"},
		{name: "blank reviewer", documentID: document.DocumentID, destination: filepath.Join(t.TempDir(), "out.sarif"), reviews: []FindingReview{{RunIndex: 0, ResultIndex: 0, Severity: "high", Disposition: "confirmed", Comment: "Reviewed", Reviewer: "  ", ReviewedAt: valid.ReviewedAt}}, want: "requires a reviewer"},
		{name: "blank comment", documentID: document.DocumentID, destination: filepath.Join(t.TempDir(), "out.sarif"), reviews: []FindingReview{{RunIndex: 0, ResultIndex: 0, Severity: "high", Disposition: "confirmed", Comment: "  ", Reviewer: valid.Reviewer, ReviewedAt: valid.ReviewedAt}}, want: "requires a comment"},
		{name: "invalid key", documentID: document.DocumentID, destination: filepath.Join(t.TempDir(), "out.sarif"), reviews: []FindingReview{{RunIndex: 0, ResultIndex: 99, Severity: "high", Disposition: "confirmed", Comment: "Reviewed", Reviewer: valid.Reviewer, ReviewedAt: valid.ReviewedAt}}, want: "does not exist"},
		{name: "invalid timestamp", documentID: document.DocumentID, destination: filepath.Join(t.TempDir(), "out.sarif"), reviews: []FindingReview{{RunIndex: 0, ResultIndex: 0, Severity: "high", Disposition: "confirmed", Comment: "Reviewed", Reviewer: valid.Reviewer, ReviewedAt: "today"}}, want: "invalid review timestamp"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := service.ExportSARIF(test.documentID, test.destination, test.reviews)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ExportSARIF() error = %v, want substring %q", err, test.want)
			}
		})
	}
}
