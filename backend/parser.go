package backend

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

const sarifVersion = "2.1.0"

func normalizeDocument(document map[string]any, documentID, sourcePath string) (SARIFDocumentDTO, error) {
	version, ok := stringValue(document["version"])
	if !ok || version == "" {
		return SARIFDocumentDTO{}, errors.New("invalid SARIF file: version is required")
	}
	if version != sarifVersion {
		return SARIFDocumentDTO{}, fmt.Errorf("unsupported SARIF version %q; only %s is supported", version, sarifVersion)
	}
	runs, ok := arrayValue(document["runs"])
	if !ok || len(runs) == 0 {
		return SARIFDocumentDTO{}, errors.New("invalid SARIF file: runs must be a non-empty array")
	}

	dto := SARIFDocumentDTO{
		DocumentID: documentID,
		FileName:   filepath.Base(sourcePath),
		SourcePath: sourcePath,
		Version:    version,
		Runs:       make([]RunSummaryDTO, 0, len(runs)),
		Findings:   make([]FindingDTO, 0),
	}
	for runIndex, rawRun := range runs {
		run, ok := objectValue(rawRun)
		if !ok {
			return SARIFDocumentDTO{}, fmt.Errorf("invalid SARIF file: run %d must be an object", runIndex+1)
		}
		toolName := toolNameForRun(run)
		runName := runNameForRun(run, runIndex, toolName)
		rawResults, exists := run["results"]
		results := []any{}
		if exists {
			var valid bool
			results, valid = arrayValue(rawResults)
			if !valid {
				return SARIFDocumentDTO{}, fmt.Errorf("invalid SARIF file: results for run %d must be an array", runIndex+1)
			}
		}
		runSummary := RunSummaryDTO{Index: runIndex, Name: runName, ToolName: toolName, FindingCount: len(results)}
		dto.Runs = append(dto.Runs, runSummary)
		for resultIndex, rawResult := range results {
			result, ok := objectValue(rawResult)
			if !ok {
				return SARIFDocumentDTO{}, fmt.Errorf("invalid SARIF file: result %d in run %d must be an object", resultIndex+1, runIndex+1)
			}
			finding := normalizeFinding(run, result, runName, toolName, runIndex, resultIndex)
			dto.Findings = append(dto.Findings, finding)
		}
	}
	dto.FindingCount = len(dto.Findings)
	return dto, nil
}

func normalizeFinding(run, result map[string]any, runName, toolName string, runIndex, resultIndex int) FindingDTO {
	rule := resolveRule(run, result)
	ruleID := firstString(result["ruleId"], nestedValue(result, "rule", "id"), rule["id"])
	ruleName := firstString(rule["name"], ruleID)
	description := messageText(rule["shortDescription"])
	if description == "" {
		description = messageText(rule["fullDescription"])
	}
	helpURI := safeHTTPURL(firstString(rule["helpUri"]))
	level := firstString(result["level"], nestedValue(rule, "defaultConfiguration", "level"))
	if level == "" {
		level = "warning"
	}
	severity, disposition, comment, reviewedAt := reviewValues(result, rule, level)
	return FindingDTO{
		Key:             FindingKey{RunIndex: runIndex, ResultIndex: resultIndex},
		RunName:         runName,
		ToolName:        toolName,
		RuleID:          ruleID,
		RuleName:        ruleName,
		RuleDescription: description,
		HelpURI:         helpURI,
		Message:         resultMessage(result, rule),
		SARIFLevel:      level,
		Severity:        severity,
		Disposition:     disposition,
		Comment:         comment,
		ReviewedAt:      reviewedAt,
		Location:        primaryLocation(run, result),
	}
}

func toolNameForRun(run map[string]any) string {
	name := firstString(nestedValue(run, "tool", "driver", "name"))
	if name == "" {
		return "Unknown tool"
	}
	return name
}

func runNameForRun(run map[string]any, index int, toolName string) string {
	name := firstString(nestedValue(run, "automationDetails", "id"), nestedValue(run, "automationDetails", "guid"))
	if name != "" {
		return name
	}
	if toolName != "Unknown tool" {
		return toolName + " run"
	}
	return fmt.Sprintf("Run %d", index+1)
}

func resolveRule(run, result map[string]any) map[string]any {
	driver, _ := objectValue(nestedValue(run, "tool", "driver"))
	components := []map[string]any{driver}
	if extensions, ok := arrayValue(nestedValue(run, "tool", "extensions")); ok {
		for _, raw := range extensions {
			if component, ok := objectValue(raw); ok {
				components = append(components, component)
			}
		}
	}

	selected := driver
	if componentRef, ok := objectValue(nestedValue(result, "rule", "toolComponent")); ok {
		if index, ok := intValue(componentRef["index"]); ok && index >= 0 && index+1 < len(components) {
			selected = components[index+1]
		} else {
			name := firstString(componentRef["name"])
			guid := firstString(componentRef["guid"])
			for _, component := range components {
				if (name != "" && firstString(component["name"]) == name) || (guid != "" && firstString(component["guid"]) == guid) {
					selected = component
					break
				}
			}
		}
	}
	rules, _ := arrayValue(selected["rules"])
	if index, ok := intValue(firstNonNil(nestedValue(result, "rule", "index"), result["ruleIndex"])); ok && index >= 0 && index < len(rules) {
		if rule, ok := objectValue(rules[index]); ok {
			return rule
		}
	}
	ruleID := firstString(result["ruleId"], nestedValue(result, "rule", "id"))
	for _, raw := range rules {
		if rule, ok := objectValue(raw); ok && firstString(rule["id"]) == ruleID {
			return rule
		}
	}
	for _, component := range components {
		componentRules, _ := arrayValue(component["rules"])
		for _, raw := range componentRules {
			if rule, ok := objectValue(raw); ok && firstString(rule["id"]) == ruleID {
				return rule
			}
		}
	}
	return map[string]any{}
}

func resultMessage(result, rule map[string]any) string {
	message, _ := objectValue(result["message"])
	if text := messageText(message); text != "" {
		return text
	}
	id := firstString(message["id"])
	if id == "" {
		return "No message provided"
	}
	template := messageText(nestedValue(rule, "messageStrings", id))
	if template == "" {
		return id
	}
	arguments, _ := arrayValue(message["arguments"])
	for index, argument := range arguments {
		template = strings.ReplaceAll(template, "{"+strconv.Itoa(index)+"}", firstString(argument))
	}
	return template
}

func messageText(value any) string {
	message, ok := objectValue(value)
	if !ok {
		return ""
	}
	return firstString(message["text"], message["markdown"])
}

func primaryLocation(run, result map[string]any) LocationDTO {
	locations, ok := arrayValue(result["locations"])
	if !ok || len(locations) == 0 {
		return LocationDTO{}
	}
	physical, _ := objectValue(nestedValue(locations[0], "physicalLocation"))
	artifact, _ := objectValue(physical["artifactLocation"])
	region, _ := objectValue(physical["region"])
	snippet := messageText(region["snippet"])
	if snippet == "" {
		if context, ok := objectValue(physical["contextRegion"]); ok {
			snippet = messageText(context["snippet"])
		}
	}
	uri := firstString(artifact["uri"])
	if baseID := firstString(artifact["uriBaseId"]); baseID != "" {
		uri = resolveArtifactURI(run, baseID, uri, map[string]bool{})
	}
	return LocationDTO{
		URI: uri, StartLine: intOrZero(region["startLine"]), StartColumn: intOrZero(region["startColumn"]),
		EndLine: intOrZero(region["endLine"]), EndColumn: intOrZero(region["endColumn"]), Snippet: snippet,
	}
}

func resolveArtifactURI(run map[string]any, baseID, child string, visited map[string]bool) string {
	if visited[baseID] {
		return child
	}
	visited[baseID] = true
	base, ok := objectValue(nestedValue(run, "originalUriBaseIds", baseID))
	if !ok {
		return child
	}
	baseURI := firstString(base["uri"])
	if parent := firstString(base["uriBaseId"]); parent != "" {
		baseURI = resolveArtifactURI(run, parent, baseURI, visited)
	}
	parsedBase, baseErr := url.Parse(baseURI)
	parsedChild, childErr := url.Parse(child)
	if baseErr == nil && childErr == nil {
		return parsedBase.ResolveReference(parsedChild).String()
	}
	return baseURI + child
}

func safeHTTPURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return parsed.String()
}
