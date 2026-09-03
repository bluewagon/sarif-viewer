package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type SARIFService struct {
	mu      sync.RWMutex
	current *loadedSARIF
}

func NewSARIFService() *SARIFService {
	return &SARIFService{}
}

func (s *SARIFService) LoadSARIF(path string, source SourceSelectionDTO) (SARIFDocumentDTO, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return SARIFDocumentDTO{}, errors.New("no SARIF file was selected")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return SARIFDocumentDTO{}, fmt.Errorf("read SARIF file: %w", err)
	}
	document, err := decodeJSONObject(data)
	if err != nil {
		return SARIFDocumentDTO{}, fmt.Errorf("parse SARIF file: %w", err)
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return SARIFDocumentDTO{}, fmt.Errorf("resolve SARIF path: %w", err)
	}
	digest := sha256.Sum256(data)
	documentID := hex.EncodeToString(digest[:])
	dto, err := normalizeDocument(document, documentID, absolutePath)
	if err != nil {
		return SARIFDocumentDTO{}, err
	}

	provider, cleanupPath, err := prepareSource(source, document)
	if err != nil {
		return SARIFDocumentDTO{}, err
	}
	keepSource := false
	if cleanupPath != "" {
		defer func() {
			if !keepSource {
				_ = os.RemoveAll(cleanupPath)
			}
		}()
	}
	enrichSnippets(document, &dto, provider, source.ContextLines)

	s.mu.Lock()
	previous := s.current
	s.current = &loadedSARIF{document: document, sourcePath: absolutePath, dto: dto, cleanupPath: cleanupPath}
	s.mu.Unlock()
	keepSource = true
	if previous != nil && previous.cleanupPath != "" && previous.cleanupPath != cleanupPath {
		_ = os.RemoveAll(previous.cleanupPath)
	}
	return dto, nil
}

func prepareSource(source SourceSelectionDTO, document map[string]any) (sourceProvider, string, error) {
	if source.ContextLines < 0 || source.ContextLines > 20 {
		return nil, "", errors.New("context lines must be between 0 and 20")
	}
	switch strings.ToLower(strings.TrimSpace(source.Kind)) {
	case "", "none":
		return nil, "", nil
	case "local":
		provider, err := newLocalSource(source.Location)
		return provider, "", err
	case "git":
		temporaryRoot, err := os.MkdirTemp("", "sastafras-source-")
		if err != nil {
			return nil, "", fmt.Errorf("create temporary source folder: %w", err)
		}
		provider, err := newGitSource(source.Location, temporaryRoot, document, source.GitAuthentication)
		if err != nil {
			_ = os.RemoveAll(temporaryRoot)
			return nil, "", err
		}
		return provider, temporaryRoot, nil
	default:
		return nil, "", errors.New("source kind must be none, local, or git")
	}
}

func (s *SARIFService) ServiceShutdown() error {
	s.mu.Lock()
	current := s.current
	s.current = nil
	s.mu.Unlock()
	if current != nil && current.cleanupPath != "" {
		return os.RemoveAll(current.cleanupPath)
	}
	return nil
}

func (s *SARIFService) ExportSARIF(documentID, destination string, reviews []FindingReview) error {
	s.mu.RLock()
	current := s.current
	if current == nil || current.dto.DocumentID != documentID {
		s.mu.RUnlock()
		return errors.New("the loaded SARIF document has changed; reopen it before exporting")
	}
	document, err := cloneJSONObject(current.document)
	sourcePath := current.sourcePath
	originalFindings := append([]FindingDTO(nil), current.dto.Findings...)
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("prepare SARIF export: %w", err)
	}

	destination = strings.TrimSpace(destination)
	if destination == "" {
		return errors.New("no export destination was selected")
	}
	absoluteDestination, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("resolve export path: %w", err)
	}
	if samePath(sourcePath, absoluteDestination) {
		return errors.New("the reviewed SARIF must be saved to a different file")
	}

	originalByKey := make(map[FindingKey]FindingDTO, len(originalFindings))
	for _, finding := range originalFindings {
		originalByKey[finding.Key] = finding
	}
	seen := make(map[FindingKey]bool, len(reviews))
	for _, review := range reviews {
		key := FindingKey{RunIndex: review.RunIndex, ResultIndex: review.ResultIndex}
		if seen[key] {
			return fmt.Errorf("finding %d:%d was supplied more than once", key.RunIndex, key.ResultIndex)
		}
		seen[key] = true
		original, ok := originalByKey[key]
		if !ok {
			return fmt.Errorf("finding %d:%d does not exist", key.RunIndex, key.ResultIndex)
		}
		if !validSeverities[review.Severity] {
			return fmt.Errorf("finding %d:%d has an invalid severity", key.RunIndex, key.ResultIndex)
		}
		if !validDispositions[review.Disposition] {
			return fmt.Errorf("finding %d:%d has an invalid disposition", key.RunIndex, key.ResultIndex)
		}
		review.Comment = strings.TrimSpace(review.Comment)
		if review.Comment == "" {
			return fmt.Errorf("finding %d:%d requires a comment", key.RunIndex, key.ResultIndex)
		}
		if _, err := time.Parse(time.RFC3339, review.ReviewedAt); err != nil {
			return fmt.Errorf("finding %d:%d has an invalid review timestamp", key.RunIndex, key.ResultIndex)
		}
		result, err := resultAt(document, key)
		if err != nil {
			return err
		}
		if review.Severity != original.Severity {
			result["level"] = sarifLevelForSeverity(review.Severity)
		}
		mergeReviewMetadata(result, review)
	}

	if err := writeJSONAtomic(absoluteDestination, document); err != nil {
		return fmt.Errorf("export reviewed SARIF: %w", err)
	}
	return nil
}
