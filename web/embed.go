// Package web embeds the built React SPA into the binary:
// one Go binary, whole product included. Run `make web` (or the Docker
// build) to produce dist/ before compiling.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS serves dist/ at the web root.
var FS fs.FS

func init() {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	FS = sub
}
