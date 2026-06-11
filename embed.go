// Package gascitypacks exposes the registry pack content as embedded
// filesystems so consumers (the gc binary) can depend on released pack
// bytes through the Go module system instead of vendoring checked-in
// copies. The embedded trees are the same content the registry releases
// hash; nested adapter modules (slack-*) are not embedded.
package gascitypacks

import (
	"embed"
	"io/fs"
)

// packsFS embeds the gastown pack tree. Additional packs join this
// pattern list as consumers need them.
//
//go:embed all:gastown
var packsFS embed.FS

// Gastown returns the gastown pack content rooted at the pack directory
// (pack.toml at the top level), matching the layout consumers compose.
func Gastown() fs.FS {
	sub, err := fs.Sub(packsFS, "gastown")
	if err != nil {
		// fs.Sub only fails on an invalid path literal; "gastown" is
		// embedded above, so this is unreachable at runtime.
		panic(err)
	}
	return sub
}
