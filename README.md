# SARIF Viewer

SARIF Viewer is a local desktop application for reviewing findings in SARIF 2.1.0 files. It combines findings from every run, provides search and triage filters, and lets a reviewer assign a security severity, status, and required rationale before exporting a separate reviewed SARIF file.

SARIF Viewer supports macOS and Windows desktop builds.

When opening a report, reviewers can select a local source folder, clone a Git repository, or continue with snippets embedded in the SARIF file. Source-backed snippets are generated for display only and are not added to exported reports.

SARIF files, local source, and review data are not uploaded by SARIF Viewer. Choosing a Git repository contacts the specified Git host through the system Git client. Imports can use existing public, SSH, or system-managed credentials, or a one-time personal access token for GitHub, Bitbucket Data Center, or Azure DevOps. PATs are passed only to the clone and revision-fetch processes, are never placed in repository URLs or Git configuration files, and are not retained after the import attempt. Temporary clones are removed when another report is loaded or the application exits.

## Development

Requirements:

- Go 1.25 or newer
- Node.js and npm
- Wails 3 CLI (`v3.0.0-beta.16` for this project)

Install frontend dependencies and start the Wails development environment:

```sh
cd frontend
npm install
cd ..
wails3 dev
```

## Repository layout

- `main.go` is the thin Wails entrypoint that embeds the built frontend.
- `backend/` contains the application setup, SARIF service, models, parsing, review, JSON, and storage code.
- `frontend/` contains the React application and generated Wails bindings.

## Tests and builds

```sh
go test ./...
cd frontend && npm test && npm run build
cd .. && wails3 build
```

On Apple Silicon, cross-build the typical 64-bit Windows executable with:

```sh
wails3 build GOOS=windows GOARCH=amd64
```

The generated TypeScript bindings in `frontend/bindings` are refreshed by the Wails build tasks. To regenerate them directly:

```sh
wails3 generate bindings -clean=true -ts -i
```

## Review metadata

Reviewed results retain their original SARIF content and add an application-owned property bag:

```json
{
  "properties": {
    "threathound/reviewStatus": {
      "severity": "high",
      "status": "confirmed",
      "rationale": "Validated the unsafe data flow.",
      "reviewer": "security-reviewer",
      "reviewedAt": "2026-09-02T12:30:00Z"
    }
  }
}
```

Changing the security tier also updates the standard SARIF `result.level` to provide a coarse severity mapping for other SARIF consumers.
