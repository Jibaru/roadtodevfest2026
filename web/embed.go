// Package web embeds the audience and stage pages into the binary:
// one Go binary, whole show included.
package web

import "embed"

//go:embed audience.html stage.html
var FS embed.FS
