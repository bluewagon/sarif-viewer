# Sastafras

Sastafras is a local desktop application for reviewing findings in SARIF 2.1.0 files. It combines findings from every run, provides search and triage filters, and lets a reviewer assign a security severity, disposition, and required comment before exporting a separate reviewed SARIF file.

The application does not upload scan data or read source files referenced by SARIF artifact locations.

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

The generated TypeScript bindings in `frontend/bindings` are refreshed by the Wails build tasks. To regenerate them directly:

```sh
wails3 generate bindings -clean=true -ts -i
```

## Review metadata

Reviewed results retain their original SARIF content and add an application-owned property bag:

```json
{
  "properties": {
    "sastafras": {
      "severity": "high",
      "disposition": "confirmed",
      "comment": "Validated the unsafe data flow.",
      "reviewedAt": "2026-09-02T12:30:00Z"
    }
  }
}
```

Changing the security tier also updates the standard SARIF `result.level` to provide a coarse severity mapping for other SARIF consumers.
