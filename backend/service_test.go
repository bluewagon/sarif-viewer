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
    "unknownResult":[1,2,3],
    "properties":{"owner":"security","sastafras":{"custom":"keep"}}
  }]}]
}`)
	document, err := service.LoadSARIF(source, SourceSelectionDTO{Kind: "none", ContextLines: 3})
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "reviewed.sarif")
	review := FindingReview{
		RunIndex: 0, ResultIndex: 0, Severity: "critical", Disposition: "false-positive",
		Comment: "  Accepted test fixture  ", ReviewedAt: "2026-09-02T12:30:00Z",
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
	metadata, _ := objectValue(nestedValue(result, "properties", "sastafras"))
	if metadata["custom"] != "keep" || metadata["comment"] != "Accepted test fixture" || metadata["disposition"] != "false-positive" {
		t.Fatalf("review metadata was not merged: %+v", metadata)
	}
}

func TestExportSARIFValidation(t *testing.T) {
	service := NewSARIFService()
	source := writeTestFile(t, "source.sarif", `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"Tool"}},"results":[{"message":{"text":"Finding"}}]}]}`)
	document, err := service.LoadSARIF(source, SourceSelectionDTO{Kind: "none", ContextLines: 3})
	if err != nil {
		t.Fatal(err)
	}
	valid := FindingReview{RunIndex: 0, ResultIndex: 0, Severity: "high", Disposition: "confirmed", Comment: "Reviewed", ReviewedAt: "2026-09-02T12:30:00Z"}
	tests := []struct {
		name        string
		documentID  string
		destination string
		reviews     []FindingReview
		want        string
	}{
		{name: "stale document", documentID: "wrong", destination: filepath.Join(t.TempDir(), "out.sarif"), reviews: []FindingReview{valid}, want: "document has changed"},
		{name: "same path", documentID: document.DocumentID, destination: source, reviews: []FindingReview{valid}, want: "different file"},
		{name: "blank comment", documentID: document.DocumentID, destination: filepath.Join(t.TempDir(), "out.sarif"), reviews: []FindingReview{{RunIndex: 0, ResultIndex: 0, Severity: "high", Disposition: "confirmed", Comment: "  ", ReviewedAt: valid.ReviewedAt}}, want: "requires a comment"},
		{name: "invalid key", documentID: document.DocumentID, destination: filepath.Join(t.TempDir(), "out.sarif"), reviews: []FindingReview{{RunIndex: 0, ResultIndex: 99, Severity: "high", Disposition: "confirmed", Comment: "Reviewed", ReviewedAt: valid.ReviewedAt}}, want: "does not exist"},
		{name: "invalid timestamp", documentID: document.DocumentID, destination: filepath.Join(t.TempDir(), "out.sarif"), reviews: []FindingReview{{RunIndex: 0, ResultIndex: 0, Severity: "high", Disposition: "confirmed", Comment: "Reviewed", ReviewedAt: "today"}}, want: "invalid review timestamp"},
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
