// Package web holds the Dashboard files embedded into the executable.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var static embed.FS

func FS() fs.FS {
	sub, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
