// Package plugins holds the plugins Vibe comes with, a folder each, which
// the app embeds and runs as it runs the plugins of ~/.vibe/plugins.
package plugins

import "embed"

// FS is the bundled plugins, a folder each.
//
//go:embed */plugin.json */*.ts
var FS embed.FS
