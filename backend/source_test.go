package backend

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPrepareGitAuthenticationFormatsProviderCredentials(t *testing.T) {
	tests := []struct {
		name     string
		location string
		input    GitAuthenticationDTO
		want     string
	}{
		{
			name:     "GitHub username and PAT",
			location: "https://github.com/example/project.git",
			input:    GitAuthenticationDTO{Provider: "github", Username: "octocat", PersonalAccessToken: "github-token"},
			want:     "octocat:github-token",
		},
		{
			name:     "Bitbucket Data Center username and PAT",
			location: "https://bitbucket.example.com/scm/project/repository.git",
			input:    GitAuthenticationDTO{Provider: "bitbucket", Username: "reviewer", PersonalAccessToken: "bitbucket-token"},
			want:     "reviewer:bitbucket-token",
		},
		{
			name:     "Azure DevOps PAT without username",
			location: "https://dev.azure.com/organization/project/_git/repository",
			input:    GitAuthenticationDTO{Provider: "azure-devops", Username: "ignored", PersonalAccessToken: "azure-token"},
			want:     ":azure-token",
		},
		{
			name:     "Azure DevOps Server",
			location: "https://azure.example.com/tfs/collection/project/_git/repository",
			input:    GitAuthenticationDTO{Provider: "azure-devops", PersonalAccessToken: "server-token"},
			want:     ":server-token",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authentication, err := prepareGitAuthentication(test.location, test.input)
			if err != nil {
				t.Fatal(err)
			}
			encoded := strings.TrimPrefix(authentication.authorization, "Authorization: Basic ")
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if string(decoded) != test.want {
				t.Fatalf("credential = %q, want %q", decoded, test.want)
			}
		})
	}
}

func TestPrepareGitAuthenticationValidation(t *testing.T) {
	validGitHub := GitAuthenticationDTO{Provider: "github", Username: "octocat", PersonalAccessToken: "token"}
	tests := []struct {
		name     string
		location string
		input    GitAuthenticationDTO
		want     string
	}{
		{name: "existing authentication accepts SSH", location: "git@github.com:example/project.git", input: GitAuthenticationDTO{Provider: "existing"}},
		{name: "existing authentication rejects PAT fields", location: "https://github.com/example/project.git", input: GitAuthenticationDTO{Provider: "existing", PersonalAccessToken: "token"}, want: "select a personal access token provider"},
		{name: "unknown provider", location: "https://github.com/example/project.git", input: GitAuthenticationDTO{Provider: "other", PersonalAccessToken: "token"}, want: "provider"},
		{name: "PAT requires HTTPS", location: "http://github.com/example/project.git", input: validGitHub, want: "HTTPS"},
		{name: "PAT rejects SSH", location: "git@github.com:example/project.git", input: validGitHub, want: "HTTPS"},
		{name: "PAT rejects URL credentials", location: "https://octocat:secret@github.com/example/project.git", input: validGitHub, want: "do not include credentials"},
		{name: "GitHub requires username", location: "https://github.com/example/project.git", input: GitAuthenticationDTO{Provider: "github", PersonalAccessToken: "token"}, want: "username"},
		{name: "Bitbucket requires username", location: "https://bitbucket.example.com/scm/project/repository.git", input: GitAuthenticationDTO{Provider: "bitbucket", PersonalAccessToken: "token"}, want: "username"},
		{name: "provider requires PAT", location: "https://github.com/example/project.git", input: GitAuthenticationDTO{Provider: "github", Username: "octocat"}, want: "personal access token"},
		{name: "GitHub host validation", location: "https://git.example.com/example/project.git", input: validGitHub, want: "github.com"},
		{name: "Azure path validation", location: "https://dev.azure.com/organization/project/repository", input: GitAuthenticationDTO{Provider: "azure-devops", PersonalAccessToken: "token"}, want: "/_git/"},
		{name: "Bitbucket path validation", location: "https://bitbucket.example.com", input: GitAuthenticationDTO{Provider: "bitbucket", Username: "reviewer", PersonalAccessToken: "token"}, want: "repository path"},
		{name: "query rejected", location: "https://github.com/example/project.git?token=secret", input: validGitHub, want: "invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authentication, err := prepareGitAuthentication(test.location, test.input)
			if test.want == "" {
				if err != nil || authentication != nil {
					t.Fatalf("prepareGitAuthentication() = %+v, %v; want existing authentication", authentication, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("prepareGitAuthentication() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestGitPATEnvironmentIsProcessScopedAndNonInteractive(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "http.extraHeader")
	t.Setenv("GIT_CONFIG_VALUE_0", "Authorization: stale")
	authentication, err := prepareGitAuthentication("https://github.com/example/project.git", GitAuthenticationDTO{
		Provider: "github", Username: "octocat", PersonalAccessToken: "one-time-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	environment := gitEnvironment(authentication)
	joined := strings.Join(environment, "\n")
	for _, expected := range []string{
		"GIT_CONFIG_COUNT=4",
		"GIT_CONFIG_VALUE_1=" + authentication.authorization,
		"GIT_CONFIG_KEY_2=credential.helper",
		"GIT_CONFIG_KEY_3=credential.interactive",
		"GIT_CONFIG_VALUE_3=false",
		"GCM_INTERACTIVE=Never",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("Git environment does not contain %q", expected)
		}
	}
	if strings.Contains(joined, "Authorization: stale") {
		t.Fatal("Git environment retained inherited authentication")
	}
	if got := strings.Join(gitEnvironment(nil), "\n"); strings.Contains(got, "one-time-token") {
		t.Fatal("unauthenticated Git environment retained the PAT")
	}
	output, err := runGitWithAuthentication("", authentication, "config", "--get-all", "http.extraHeader")
	if err != nil {
		t.Fatalf("Git rejected process-scoped authentication configuration: %v: %s", err, output)
	}
	if strings.TrimSpace(string(output)) != authentication.authorization {
		t.Fatalf("Git HTTP authorization header = %q, want %q", strings.TrimSpace(string(output)), authentication.authorization)
	}
}

func TestSanitizedGitErrorRedactsPATAndFormatsAuthenticationFailure(t *testing.T) {
	authentication, err := prepareGitAuthentication("https://github.com/example/project.git", GitAuthenticationDTO{
		Provider: "github", Username: "octocat", PersonalAccessToken: "super-secret-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded := strings.TrimPrefix(authentication.authorization, "Authorization: Basic ")
	detail := fmt.Sprintf("Authentication failed for https://github.com/example/project.git %s %s", authentication.authorization, encoded)
	got := sanitizedGitError("clone Git repository", []byte(detail), errors.New("exit status 128"), "https://github.com/example/project.git", authentication).Error()
	for _, secret := range []string{"super-secret-token", encoded, authentication.authorization, "https://github.com/example/project.git"} {
		if strings.Contains(got, secret) {
			t.Fatalf("sanitized error leaked %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "GitHub authentication failed") || !strings.Contains(got, "SSO") {
		t.Fatalf("unexpected authentication error: %s", got)
	}
}

func TestGitPATAuthenticatesCloneAndRevisionFetchWithoutPersistence(t *testing.T) {
	work := t.TempDir()
	runTestGit(t, work, "init", "--quiet")
	runTestGit(t, work, "config", "user.email", "test@example.com")
	runTestGit(t, work, "config", "user.name", "Test")
	writeSourceFile(t, work, "src/value.txt", "first\n")
	runTestGit(t, work, "add", "src/value.txt")
	runTestGit(t, work, "commit", "--quiet", "-m", "first")

	serverRoot := t.TempDir()
	bare := filepath.Join(serverRoot, "repository.git")
	runTestGit(t, work, "clone", "--quiet", "--bare", ".", bare)
	runTestGit(t, bare, "update-server-info")

	authentication, err := prepareGitAuthentication("https://bitbucket.example.com/repository.git", GitAuthenticationDTO{
		Provider: "bitbucket", Username: "reviewer", PersonalAccessToken: "one-time-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	var requestsMu sync.Mutex
	authenticatedRequests := 0
	files := http.FileServer(http.Dir(serverRoot))
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != strings.TrimPrefix(authentication.authorization, "Authorization: ") {
			writer.Header().Set("WWW-Authenticate", `Basic realm="Git"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		requestsMu.Lock()
		authenticatedRequests++
		requestsMu.Unlock()
		files.ServeHTTP(writer, request)
	}))
	defer server.Close()
	t.Setenv("GIT_SSL_NO_VERIFY", "true")

	temporaryRoot := t.TempDir()
	provider, err := newGitSource(server.URL+"/repository.git", temporaryRoot, map[string]any{"runs": []any{}}, GitAuthenticationDTO{
		Provider: "bitbucket", Username: "reviewer", PersonalAccessToken: "one-time-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(provider.repository, "config"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range authentication.secrets {
		if strings.Contains(string(config), secret) {
			t.Fatalf("temporary Git configuration persisted credential %q", secret)
		}
	}

	requestsMu.Lock()
	cloneRequestCount := authenticatedRequests
	requestsMu.Unlock()
	if output, err := runGitWithAuthentication(provider.repository, authentication, "fetch", "--quiet", "--", "origin", "HEAD"); err != nil {
		t.Fatalf("authenticated fetch failed: %v: %s", err, output)
	}
	requestsMu.Lock()
	requestCount := authenticatedRequests
	requestsMu.Unlock()
	if cloneRequestCount == 0 || requestCount <= cloneRequestCount {
		t.Fatalf("authenticated Git requests before fetch = %d, after fetch = %d", cloneRequestCount, requestCount)
	}
}

func TestLoadSARIFGeneratesLocalSnippetWithoutChangingExport(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "src/app.go", "one\ntwo\nthree\nfour\nfive\nsix\nseven\n")
	sarif := writeTestFile(t, "source.sarif", `{
  "version":"2.1.0",
  "runs":[{
    "originalUriBaseIds":{"ROOT":{"uri":"file:///scanner/workspace/"}},
    "tool":{"driver":{"name":"Tool"}},
    "results":[{"ruleId":"R1","message":{"text":"Finding"},"locations":[{"physicalLocation":{
      "artifactLocation":{"uri":"src/app.go","uriBaseId":"ROOT"},
      "region":{"startLine":4,"startColumn":1,"endLine":4,"endColumn":5,"snippet":{"text":"embedded original"}}
    }}]}]
  }]
}`)

	service := NewSARIFService()
	document, err := service.LoadSARIF(sarif, SourceSelectionDTO{Kind: "local", Location: root, ContextLines: 2})
	if err != nil {
		t.Fatal(err)
	}
	location := document.Findings[0].Location
	if location.Snippet != "two\nthree\nfour\nfive\nsix" || location.SnippetStartLine != 2 || location.SnippetOrigin != "source" || location.SnippetStatus != "generated" {
		t.Fatalf("unexpected generated location: %+v", location)
	}
	if document.SnippetSummary != (SnippetSummaryDTO{Generated: 1}) {
		t.Fatalf("unexpected snippet summary: %+v", document.SnippetSummary)
	}

	destination := filepath.Join(t.TempDir(), "reviewed.sarif")
	if err := service.ExportSARIF(document.DocumentID, destination, nil); err != nil {
		t.Fatal(err)
	}
	exported, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	exportedDocument, err := decodeJSONObject(exported)
	if err != nil {
		t.Fatal(err)
	}
	result, err := resultAt(exportedDocument, FindingKey{RunIndex: 0, ResultIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	locations, _ := arrayValue(result["locations"])
	if got := firstString(nestedValue(locations[0], "physicalLocation", "region", "snippet", "text")); got != "embedded original" {
		t.Fatalf("exported snippet = %q, want original embedded snippet", got)
	}
}

func TestLocalSourceResolutionAndFailures(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "src/space name.go", "package main\n")
	writeSourceFile(t, root, "one/shared.go", "one\n")
	writeSourceFile(t, root, "two/shared.go", "two\n")
	writeSourceFile(t, root, "binary.dat", string([]byte{'a', 0, 'b'}))
	writeSourceFile(t, root, "large.go", strings.Repeat("x", maxSourceFileSize+1))
	outside := writeSourceFile(t, t.TempDir(), "outside.go", "secret\n")
	if err := os.Symlink(outside, filepath.Join(root, "escape.go")); err != nil {
		t.Fatal(err)
	}
	provider, err := newLocalSource(root)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		raw        string
		resolved   string
		want       string
		wantStatus string
	}{
		{name: "relative percent encoded", raw: "src/space%20name.go", want: "package main\n"},
		{name: "absolute suffix", raw: "file:///scanner/workspace/src/space%20name.go", want: "package main\n"},
		{name: "windows suffix", raw: `C:\scanner\workspace\src\space name.go`, want: "package main\n"},
		{name: "ambiguous basename", raw: "file:///scanner/shared.go", wantStatus: "ambiguous"},
		{name: "traversal", raw: "../outside.go", wantStatus: "outside-root"},
		{name: "escaping symlink", raw: "escape.go", wantStatus: "outside-root"},
		{name: "missing", raw: "src/missing.go", wantStatus: "not-found"},
		{name: "binary", raw: "binary.dat", wantStatus: "binary"},
		{name: "oversized", raw: "large.go", wantStatus: "too-large"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := provider.readFile(sourceRequest{rawURI: test.raw, resolvedURI: test.resolved})
			if test.wantStatus != "" {
				if err == nil || snippetStatus(err) != test.wantStatus {
					t.Fatalf("readFile() error = %v status %q, want %q", err, snippetStatus(err), test.wantStatus)
				}
				return
			}
			if err != nil || string(data) != test.want {
				t.Fatalf("readFile() = %q, %v; want %q", data, err, test.want)
			}
		})
	}
}

func TestBuildSnippetContextAndLineValidation(t *testing.T) {
	data := []byte("one\ntwo\nthree\nfour\nfive\n")
	tests := []struct {
		name       string
		start      int
		end        int
		context    int
		want       string
		wantStart  int
		wantStatus string
	}{
		{name: "zero context", start: 2, end: 3, want: "two\nthree", wantStart: 2},
		{name: "clamps boundaries", start: 1, end: 1, context: 3, want: "one\ntwo\nthree\nfour", wantStart: 1},
		{name: "missing end means start", start: 5, context: 3, want: "two\nthree\nfour\nfive", wantStart: 2},
		{name: "start out of range", start: 6, wantStatus: "invalid-line"},
		{name: "end out of range", start: 4, end: 6, wantStatus: "invalid-line"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snippet, start, err := buildSnippet(data, test.start, test.end, test.context)
			if test.wantStatus != "" {
				if err == nil || snippetStatus(err) != test.wantStatus {
					t.Fatalf("buildSnippet() error = %v, want %q", err, test.wantStatus)
				}
				return
			}
			if err != nil || snippet != test.want || start != test.wantStart {
				t.Fatalf("buildSnippet() = %q, %d, %v; want %q, %d", snippet, start, err, test.want, test.wantStart)
			}
		})
	}
}

func TestGitSourceUsesRunRevisionsAndCleansUp(t *testing.T) {
	repository := t.TempDir()
	runTestGit(t, repository, "init", "--quiet")
	runTestGit(t, repository, "config", "user.email", "test@example.com")
	runTestGit(t, repository, "config", "user.name", "Test")
	writeSourceFile(t, repository, "src/value.txt", "old value\n")
	runTestGit(t, repository, "add", "src/value.txt")
	runTestGit(t, repository, "commit", "--quiet", "-m", "old")
	oldRevision := strings.TrimSpace(runTestGit(t, repository, "rev-parse", "HEAD"))
	writeSourceFile(t, repository, "src/value.txt", "new value\n")
	runTestGit(t, repository, "commit", "--quiet", "-am", "new")
	newRevision := strings.TrimSpace(runTestGit(t, repository, "rev-parse", "HEAD"))

	runJSON := func(revision string) string {
		provenance := ""
		if revision != "" {
			provenance = fmt.Sprintf(`"versionControlProvenance":[{"revisionId":%q}],`, revision)
		}
		return fmt.Sprintf(`{%s"tool":{"driver":{"name":"Tool"}},"results":[{"message":{"text":"Finding"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"src/value.txt"},"region":{"startLine":1}}}]}]}`, provenance)
	}
	sarif := writeTestFile(t, "git.sarif", fmt.Sprintf(`{"version":"2.1.0","runs":[%s,%s,%s]}`, runJSON(oldRevision), runJSON(newRevision), runJSON("")))
	service := NewSARIFService()
	document, err := service.LoadSARIF(sarif, SourceSelectionDTO{Kind: "git", Location: (&url.URL{Scheme: "file", Path: repository}).String(), ContextLines: 0})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{document.Findings[0].Location.Snippet, document.Findings[1].Location.Snippet, document.Findings[2].Location.Snippet}
	if fmt.Sprint(got) != fmt.Sprint([]string{"old value", "new value", "new value"}) {
		t.Fatalf("Git snippets = %v", got)
	}
	cleanup := service.current.cleanupPath
	if cleanup == "" {
		t.Fatal("Git source did not register cleanup path")
	}
	if err := service.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cleanup); !os.IsNotExist(err) {
		t.Fatalf("temporary Git source still exists: %v", err)
	}
}

func TestFailedSourceLoadKeepsCurrentDocument(t *testing.T) {
	service := NewSARIFService()
	first := writeTestFile(t, "first.sarif", `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"First"}}}]}`)
	document, err := service.LoadSARIF(first, SourceSelectionDTO{Kind: "none", ContextLines: 3})
	if err != nil {
		t.Fatal(err)
	}
	second := writeTestFile(t, "second.sarif", `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"Second"}}}]}`)
	if _, err := service.LoadSARIF(second, SourceSelectionDTO{Kind: "git", Location: "not a URI", ContextLines: 3}); err == nil {
		t.Fatal("LoadSARIF() succeeded with invalid Git URI")
	}
	if service.current == nil || service.current.dto.DocumentID != document.DocumentID {
		t.Fatal("failed import replaced the current document")
	}
}

func writeSourceFile(t *testing.T, root, relative, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runTestGit(t *testing.T, repository string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
	return string(output)
}
