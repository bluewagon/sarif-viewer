package main

import (
	"embed"
	"log"

	"sastafras/backend"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := backend.Run(assets); err != nil {
		log.Fatal(err)
	}
}
