package backend

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxSourceFileSize = 10 << 20

var windowsDrivePath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
var scpGitURI = regexp.MustCompile(`^[^/@:]+@[^/:]+:.+`)

type sourceRequest struct {
	runIndex    int
	rawURI      string
	resolvedURI string
}

type sourceProvider interface {
	readFile(sourceRequest) ([]byte, error)
	cleanupPath() string
}

type snippetError struct {
	status string
	err    error
}

func (e *snippetError) Error() string { return e.err.Error() }
func (e *snippetError) Unwrap() error { return e.err }

func newSnippetError(status, message string) error {
	return &snippetError{status: status, err: errors.New(message)}
}

func snippetStatus(err error) string {
	var target *snippetError
	if errors.As(err, &target) {
		return target.status
	}
	return "unreadable"
}

type localSource struct {
	root  string
	paths []string
	set   map[string]bool
}

func newLocalSource(location string) (*localSource, error) {
	location = strings.TrimSpace(location)
	if location == "" {
		return nil, errors.New("choose a local source folder")
	}
	absolute, err := filepath.Abs(location)
	if err != nil {
		return nil, fmt.Errorf("resolve source folder: %w", err)
	}
	root, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("open source folder: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("open source folder: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("the selected source location is not a folder")
	}

	provider := &localSource{root: root, set: map[string]bool{}}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == root {
				return walkErr
			}
			return nil
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == ".git" || entry.Name() == ".hg" || entry.Name() == ".svn") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() || entry.Type()&os.ModeSymlink != 0 {
			relative, relErr := filepath.Rel(root, path)
			if relErr == nil {
				normalized := filepath.ToSlash(relative)
				provider.paths = append(provider.paths, normalized)
				provider.set[normalized] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("index source folder: %w", err)
	}
	sort.Strings(provider.paths)
	return provider, nil
}

func (s *localSource) cleanupPath() string { return "" }

func (s *localSource) readFile(request sourceRequest) ([]byte, error) {
	relative, err := resolveRepositoryPath(request.rawURI, request.resolvedURI, s.paths, s.set)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(s.root, filepath.FromSlash(relative))
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, newSnippetError("not-found", "source file was not found")
	}
	if !pathWithinRoot(s.root, resolved) {
		return nil, newSnippetError("outside-root", "source file resolves outside the selected folder")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return nil, newSnippetError("unreadable", "source path is not a regular file")
	}
	if info.Size() > maxSourceFileSize {
		return nil, newSnippetError("too-large", "source file is larger than 10 MiB")
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, newSnippetError("unreadable", "source file could not be read")
	}
	return validateSourceText(data)
}

type gitSource struct {
	repository string
	runRefs    map[int]string
	trees      map[string]*gitTree
}

type gitTree struct {
	paths []string
	set   map[string]bool
}

type gitAuthentication struct {
	provider      string
	authorization string
	secrets       []string
}

func newGitSource(location, temporaryRoot string, document map[string]any, input GitAuthenticationDTO) (*gitSource, error) {
	location = strings.TrimSpace(location)
	if err := validateGitLocation(location); err != nil {
		return nil, err
	}
	authentication, err := prepareGitAuthentication(location, input)
	if err != nil {
		return nil, err
	}
	repository := filepath.Join(temporaryRoot, "repository.git")
	if output, err := runGitWithAuthentication("", authentication, "clone", "--bare", "--filter=blob:none", "--quiet", "--", location, repository); err != nil {
		return nil, sanitizedGitError("clone Git repository", output, err, location, authentication)
	}
	provider := &gitSource{repository: repository, runRefs: map[int]string{}, trees: map[string]*gitTree{}}
	runs, _ := arrayValue(document["runs"])
	for runIndex, rawRun := range runs {
		run, _ := objectValue(rawRun)
		requested := versionControlRef(run)
		commit, err := provider.resolveCommit(requested, location, authentication)
		if err != nil {
			return nil, fmt.Errorf("resolve source revision for run %d: %w", runIndex+1, err)
		}
		provider.runRefs[runIndex] = commit
		if _, exists := provider.trees[commit]; !exists {
			tree, err := provider.loadTree(commit)
			if err != nil {
				return nil, err
			}
			provider.trees[commit] = tree
		}
	}
	return provider, nil
}

func (s *gitSource) cleanupPath() string { return filepath.Dir(s.repository) }

func (s *gitSource) resolveCommit(requested, location string, authentication *gitAuthentication) (string, error) {
	if requested != "" {
		if output, err := runGit(s.repository, "rev-parse", "--verify", "--end-of-options", requested+"^{commit}"); err == nil {
			return strings.TrimSpace(string(output)), nil
		}
		output, err := runGitWithAuthentication(s.repository, authentication, "fetch", "--depth=1", "--quiet", "--", "origin", requested)
		if err != nil && strings.Contains(strings.ToLower(string(output)), "does not support shallow") {
			output, err = runGitWithAuthentication(s.repository, authentication, "fetch", "--quiet", "--", "origin", requested)
		}
		if err != nil {
			return "", sanitizedGitError("fetch Git revision", output, err, location, authentication)
		}
		return s.revParse("FETCH_HEAD", location, authentication)
	}
	return s.revParse("HEAD", location, authentication)
}

func (s *gitSource) revParse(ref, location string, authentication *gitAuthentication) (string, error) {
	output, err := runGit(s.repository, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", sanitizedGitError("resolve Git revision", output, err, location, authentication)
	}
	return strings.TrimSpace(string(output)), nil
}

func (s *gitSource) loadTree(commit string) (*gitTree, error) {
	output, err := runGit(s.repository, "ls-tree", "-r", "--name-only", "-z", commit)
	if err != nil {
		return nil, fmt.Errorf("list Git source files: %w", err)
	}
	parts := bytes.Split(output, []byte{0})
	tree := &gitTree{set: map[string]bool{}}
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		path := filepath.ToSlash(string(part))
		tree.paths = append(tree.paths, path)
		tree.set[path] = true
	}
	sort.Strings(tree.paths)
	return tree, nil
}

func (s *gitSource) readFile(request sourceRequest) ([]byte, error) {
	commit := s.runRefs[request.runIndex]
	if commit == "" {
		commit = s.runRefs[0]
	}
	tree := s.trees[commit]
	if tree == nil {
		return nil, newSnippetError("unreadable", "source revision is unavailable")
	}
	relative, err := resolveRepositoryPath(request.rawURI, request.resolvedURI, tree.paths, tree.set)
	if err != nil {
		return nil, err
	}
	output, err := runGit(s.repository, "show", commit+":"+relative)
	if err != nil {
		return nil, newSnippetError("unreadable", "source file could not be read from Git")
	}
	if len(output) > maxSourceFileSize {
		return nil, newSnippetError("too-large", "source file is larger than 10 MiB")
	}
	return validateSourceText(output)
}

func validateSourceText(data []byte) ([]byte, error) {
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return nil, newSnippetError("binary", "source file is not UTF-8 text")
	}
	return data, nil
}

func validateGitLocation(location string) error {
	if location == "" {
		return errors.New("enter a Git repository URI")
	}
	if strings.HasPrefix(location, "-") || strings.ContainsRune(location, 0) {
		return errors.New("invalid Git repository URI")
	}
	if scpGitURI.MatchString(location) {
		return nil
	}
	parsed, err := url.Parse(location)
	if err != nil || parsed.Scheme == "" {
		return errors.New("use an HTTPS, SSH, Git, file, or SCP-style Git URI")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "ssh", "git", "file":
		return nil
	default:
		return errors.New("unsupported Git repository URI scheme")
	}
}

func prepareGitAuthentication(location string, input GitAuthenticationDTO) (*gitAuthentication, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider == "" || provider == "existing" {
		if strings.TrimSpace(input.Username) != "" || strings.TrimSpace(input.PersonalAccessToken) != "" {
			return nil, errors.New("select a personal access token provider before entering Git credentials")
		}
		return nil, nil
	}
	if provider != "github" && provider != "bitbucket" && provider != "azure-devops" {
		return nil, errors.New("Git authentication provider must be existing, GitHub, Bitbucket, or Azure DevOps")
	}

	parsed, err := url.Parse(location)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
		return nil, errors.New("personal access token authentication requires an HTTPS Git repository URI")
	}
	if parsed.User != nil {
		return nil, errors.New("do not include credentials in the Git repository URI; use the username and personal access token fields")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || containsControl(location) {
		return nil, errors.New("invalid Git repository URI")
	}

	username := strings.TrimSpace(input.Username)
	token := strings.TrimSpace(input.PersonalAccessToken)
	if token == "" {
		return nil, errors.New("enter a personal access token")
	}
	if containsControl(token) {
		return nil, errors.New("personal access token contains invalid characters")
	}
	if provider == "github" || provider == "bitbucket" {
		if username == "" {
			return nil, fmt.Errorf("enter the %s username associated with the personal access token", providerDisplayName(provider))
		}
		if containsControl(username) {
			return nil, errors.New("Git username contains invalid characters")
		}
	}

	path := strings.Trim(parsed.EscapedPath(), "/")
	switch provider {
	case "github":
		if !strings.EqualFold(parsed.Hostname(), "github.com") || len(strings.Split(path, "/")) < 2 {
			return nil, errors.New("GitHub PAT imports require an https://github.com/OWNER/REPOSITORY clone URI")
		}
	case "bitbucket":
		if path == "" {
			return nil, errors.New("Bitbucket Data Center clone URI must include a repository path")
		}
	case "azure-devops":
		_, repositoryName, found := strings.Cut(strings.ToLower(parsed.Path), "/_git/")
		if !found || strings.Trim(repositoryName, "/") == "" {
			return nil, errors.New("Azure DevOps clone URI must include /_git/ followed by the repository name")
		}
		username = ""
	}

	credential := username + ":" + token
	encoded := base64.StdEncoding.EncodeToString([]byte(credential))
	authorization := "Authorization: Basic " + encoded
	return &gitAuthentication{
		provider:      provider,
		authorization: authorization,
		secrets:       []string{token, encoded, authorization},
	}, nil
}

func containsControl(value string) bool {
	return strings.IndexFunc(value, func(character rune) bool {
		return character < 0x20 || character == 0x7f
	}) >= 0
}

func providerDisplayName(provider string) string {
	switch provider {
	case "github":
		return "GitHub"
	case "bitbucket":
		return "Bitbucket"
	case "azure-devops":
		return "Azure DevOps"
	default:
		return "Git"
	}
}

func runGit(repository string, arguments ...string) ([]byte, error) {
	return runGitWithAuthentication(repository, nil, arguments...)
}

func runGitWithAuthentication(repository string, authentication *gitAuthentication, arguments ...string) ([]byte, error) {
	if repository != "" {
		arguments = append([]string{"-C", repository}, arguments...)
	}
	command := exec.Command("git", arguments...)
	command.Env = gitEnvironment(authentication)
	return command.CombinedOutput()
}

func gitEnvironment(authentication *gitAuthentication) []string {
	environment := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if authentication == nil {
		return environment
	}
	filtered := environment[:0]
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		if key == "GIT_CONFIG_COUNT" || strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered,
		"GIT_CONFIG_COUNT=4",
		"GIT_CONFIG_KEY_0=http.extraHeader",
		"GIT_CONFIG_VALUE_0=",
		"GIT_CONFIG_KEY_1=http.extraHeader",
		"GIT_CONFIG_VALUE_1="+authentication.authorization,
		"GIT_CONFIG_KEY_2=credential.helper",
		"GIT_CONFIG_VALUE_2=",
		"GIT_CONFIG_KEY_3=credential.interactive",
		"GIT_CONFIG_VALUE_3=false",
		"GCM_INTERACTIVE=Never",
	)
}

func sanitizedGitError(action string, output []byte, commandErr error, location string, authentication *gitAuthentication) error {
	detail := strings.TrimSpace(string(output))
	if location != "" {
		detail = strings.ReplaceAll(detail, location, "the provided repository")
	}
	if authentication != nil {
		for _, secret := range authentication.secrets {
			if secret != "" {
				detail = strings.ReplaceAll(detail, secret, "[redacted]")
			}
		}
		if isAuthenticationFailure(detail) {
			return errors.New(authenticationFailureMessage(authentication.provider))
		}
	}
	if detail == "" {
		detail = commandErr.Error()
	}
	return fmt.Errorf("%s: %s", action, detail)
}

func isAuthenticationFailure(detail string) bool {
	detail = strings.ToLower(detail)
	for _, marker := range []string{"authentication failed", "authorization failed", "access denied", "unauthorized", "forbidden", "could not read username", "status code: 401", "status code: 403", "error: 401", "error: 403"} {
		if strings.Contains(detail, marker) {
			return true
		}
	}
	return false
}

func authenticationFailureMessage(provider string) string {
	switch provider {
	case "github":
		return "GitHub authentication failed. Verify the username, personal access token, repository read permission, token expiry, and organization SSO authorization."
	case "bitbucket":
		return "Bitbucket Data Center authentication failed. Verify the username, personal access token, repository read permission, and token expiry."
	case "azure-devops":
		return "Azure DevOps authentication failed. Verify the personal access token, Code read permission, and token expiry."
	default:
		return "Git authentication failed. Verify the supplied credentials and repository access."
	}
}

func versionControlRef(run map[string]any) string {
	details, _ := arrayValue(run["versionControlProvenance"])
	for _, raw := range details {
		detail, ok := objectValue(raw)
		if !ok {
			continue
		}
		if ref := firstString(detail["revisionId"], detail["branch"]); ref != "" {
			return ref
		}
	}
	return ""
}

func resolveRepositoryPath(rawURI, resolvedURI string, paths []string, set map[string]bool) (string, error) {
	if path, relative, supported := artifactPath(rawURI); supported && relative && !safeRelativePath(path) {
		return "", newSnippetError("outside-root", "artifact location escapes the selected source folder")
	}
	unsafeRelative := false
	for _, value := range []string{rawURI, resolvedURI} {
		path, relative, supported := artifactPath(value)
		if !supported {
			continue
		}
		if relative {
			if !safeRelativePath(path) {
				unsafeRelative = true
				continue
			}
			if set[path] {
				return path, nil
			}
		}
	}

	bestScore := 0
	best := ""
	ambiguous := false
	for _, value := range []string{rawURI, resolvedURI} {
		artifact, relative, supported := artifactPath(value)
		if !supported || artifact == "" {
			continue
		}
		if relative && !safeRelativePath(artifact) {
			continue
		}
		for _, candidate := range paths {
			score := commonSuffixComponents(artifact, candidate)
			if score == 0 {
				continue
			}
			if score > bestScore {
				bestScore, best, ambiguous = score, candidate, false
			} else if score == bestScore && candidate != best {
				ambiguous = true
			}
		}
	}
	if bestScore == 0 {
		if unsafeRelative {
			return "", newSnippetError("outside-root", "artifact location escapes the selected source folder")
		}
		return "", newSnippetError("not-found", "source file was not found")
	}
	if ambiguous {
		return "", newSnippetError("ambiguous", "artifact location matches more than one source file")
	}
	return best, nil
}

func artifactPath(value string) (path string, relative bool, supported bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, false
	}
	if windowsDrivePath.MatchString(value) {
		path = strings.ReplaceAll(value, "\\", "/")
		return strings.TrimPrefix(path, "/"), false, true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", false, false
	}
	if parsed.Scheme != "" && !strings.EqualFold(parsed.Scheme, "file") {
		return "", false, false
	}
	path = parsed.Path
	if parsed.Scheme == "" {
		path = value
	}
	path, err = url.PathUnescape(path)
	if err != nil {
		return "", false, false
	}
	path = strings.ReplaceAll(path, "\\", "/")
	relative = !strings.HasPrefix(path, "/") && !windowsDrivePath.MatchString(path)
	path = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))), "./")
	if !relative {
		path = strings.TrimPrefix(path, "/")
	}
	return path, relative, true
}

func safeRelativePath(path string) bool {
	return path != "" && path != "." && path != ".." && !strings.HasPrefix(path, "../") && !strings.HasPrefix(path, "/")
}

func commonSuffixComponents(left, right string) int {
	leftParts := strings.Split(strings.Trim(left, "/"), "/")
	rightParts := strings.Split(strings.Trim(right, "/"), "/")
	score := 0
	for leftIndex, rightIndex := len(leftParts)-1, len(rightParts)-1; leftIndex >= 0 && rightIndex >= 0; leftIndex, rightIndex = leftIndex-1, rightIndex-1 {
		if leftParts[leftIndex] != rightParts[rightIndex] {
			break
		}
		score++
	}
	return score
}

func pathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func sourceRequestForFinding(run map[string]any, result map[string]any, runIndex int) (sourceRequest, bool) {
	locations, ok := arrayValue(result["locations"])
	if !ok || len(locations) == 0 {
		return sourceRequest{}, false
	}
	artifact, _ := objectValue(nestedValue(locations[0], "physicalLocation", "artifactLocation"))
	rawURI := firstString(artifact["uri"])
	resolvedURI := rawURI
	if baseID := firstString(artifact["uriBaseId"]); baseID != "" {
		resolvedURI = resolveArtifactURI(run, baseID, rawURI, map[string]bool{})
	}
	return sourceRequest{runIndex: runIndex, rawURI: rawURI, resolvedURI: resolvedURI}, rawURI != "" || resolvedURI != ""
}

func enrichSnippets(document map[string]any, dto *SARIFDocumentDTO, provider sourceProvider, contextLines int) {
	findingIndex := 0
	type sourceRead struct {
		data []byte
		err  error
	}
	cache := map[sourceRequest]sourceRead{}
	runs, _ := arrayValue(document["runs"])
	for runIndex, rawRun := range runs {
		run, _ := objectValue(rawRun)
		results, _ := arrayValue(run["results"])
		for _, rawResult := range results {
			if findingIndex >= len(dto.Findings) {
				break
			}
			finding := &dto.Findings[findingIndex]
			findingIndex++
			result, _ := objectValue(rawResult)
			if provider == nil {
				continue
			}
			request, ok := sourceRequestForFinding(run, result, runIndex)
			if !ok || finding.Location.StartLine < 1 {
				if finding.Location.Snippet == "" {
					finding.Location.SnippetStatus = "missing-location"
				}
				continue
			}
			read, exists := cache[request]
			if !exists {
				read.data, read.err = provider.readFile(request)
				cache[request] = read
			}
			data, err := read.data, read.err
			if err != nil {
				if finding.Location.Snippet == "" {
					finding.Location.SnippetStatus = snippetStatus(err)
				}
				continue
			}
			snippet, startLine, err := buildSnippet(data, finding.Location.StartLine, finding.Location.EndLine, contextLines)
			if err != nil {
				if finding.Location.Snippet == "" {
					finding.Location.SnippetStatus = snippetStatus(err)
				}
				continue
			}
			finding.Location.Snippet = snippet
			finding.Location.SnippetStartLine = startLine
			finding.Location.SnippetOrigin = "source"
			finding.Location.SnippetStatus = "generated"
		}
	}
	dto.SnippetSummary = summarizeSnippets(dto.Findings)
}

func buildSnippet(data []byte, startLine, endLine, contextLines int) (string, int, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if startLine < 1 || startLine > len(lines) {
		return "", 0, newSnippetError("invalid-line", "finding start line is outside the source file")
	}
	if endLine == 0 {
		endLine = startLine
	}
	if endLine < startLine || endLine > len(lines) {
		return "", 0, newSnippetError("invalid-line", "finding end line is outside the source file")
	}
	first := max(1, startLine-contextLines)
	last := min(len(lines), endLine+contextLines)
	return strings.Join(lines[first-1:last], "\n"), first, nil
}

func summarizeSnippets(findings []FindingDTO) SnippetSummaryDTO {
	var summary SnippetSummaryDTO
	for _, finding := range findings {
		switch finding.Location.SnippetOrigin {
		case "source":
			summary.Generated++
		case "sarif":
			summary.Embedded++
		default:
			summary.Unavailable++
		}
	}
	return summary
}
