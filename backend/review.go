package backend

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

var validSeverities = map[string]bool{
	"critical": true, "high": true, "medium": true, "low": true, "informational": true,
}

var validDispositions = map[string]bool{
	"unreviewed": true, "confirmed": true, "false-positive": true,
}

const reviewMetadataKey = "sarif-viewer"

func reviewValues(result, rule map[string]any, level string) (string, string, string, string) {
	metadata, _ := objectValue(nestedValue(result, "properties", reviewMetadataKey))
	severity := firstString(metadata["severity"])
	if !validSeverities[severity] {
		severity = severityFromScore(firstNonNil(nestedValue(result, "properties", "security-severity"), nestedValue(rule, "properties", "security-severity")))
	}
	if severity == "" {
		severity = severityFromLevel(level)
	}
	disposition := firstString(metadata["disposition"])
	if !validDispositions[disposition] {
		disposition = "unreviewed"
	}
	return severity, disposition, firstString(metadata["comment"]), firstString(metadata["reviewedAt"])
}

func severityFromScore(value any) string {
	var score float64
	var err error
	switch typed := value.(type) {
	case json.Number:
		score, err = typed.Float64()
	case float64:
		score = typed
	case string:
		score, err = strconv.ParseFloat(typed, 64)
	default:
		return ""
	}
	if err != nil || score < 0 || score > 10 {
		return ""
	}
	switch {
	case score >= 9:
		return "critical"
	case score >= 7:
		return "high"
	case score >= 4:
		return "medium"
	case score > 0:
		return "low"
	default:
		return "informational"
	}
}

func severityFromLevel(level string) string {
	switch strings.ToLower(level) {
	case "error":
		return "high"
	case "warning":
		return "medium"
	case "note":
		return "low"
	case "none":
		return "informational"
	default:
		return "medium"
	}
}

func sarifLevelForSeverity(severity string) string {
	switch severity {
	case "critical", "high":
		return "error"
	case "medium":
		return "warning"
	default:
		return "note"
	}
}

func resultAt(document map[string]any, key FindingKey) (map[string]any, error) {
	runs, ok := arrayValue(document["runs"])
	if !ok || key.RunIndex < 0 || key.RunIndex >= len(runs) {
		return nil, fmt.Errorf("finding %d:%d does not exist", key.RunIndex, key.ResultIndex)
	}
	run, _ := objectValue(runs[key.RunIndex])
	results, ok := arrayValue(run["results"])
	if !ok || key.ResultIndex < 0 || key.ResultIndex >= len(results) {
		return nil, fmt.Errorf("finding %d:%d does not exist", key.RunIndex, key.ResultIndex)
	}
	result, ok := objectValue(results[key.ResultIndex])
	if !ok {
		return nil, fmt.Errorf("finding %d:%d is invalid", key.RunIndex, key.ResultIndex)
	}
	return result, nil
}

func mergeReviewMetadata(result map[string]any, review FindingReview) {
	properties, ok := objectValue(result["properties"])
	if !ok {
		properties = map[string]any{}
		result["properties"] = properties
	}
	metadata, ok := objectValue(properties[reviewMetadataKey])
	if !ok {
		metadata = map[string]any{}
		properties[reviewMetadataKey] = metadata
	}
	metadata["severity"] = review.Severity
	metadata["disposition"] = review.Disposition
	metadata["comment"] = review.Comment
	metadata["reviewedAt"] = review.ReviewedAt
}
