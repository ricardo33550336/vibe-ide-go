package workspace

import (
	"slices"
	"testing"

	"vibe/internal/vfs"
)

// TestRemoteChildrenKeepSlashes has the children of a folder of a machine
// spelled as the host has them, with slashes, so that a poll, which names a
// folder as the host does, finds the listing of that folder again.
func TestRemoteChildrenKeepSlashes(t *testing.T) {
	dir := "/wsl:Ubuntu/home/pepe/proj"
	kids, dirs := sortEntries(dir, []vfs.Entry{
		{Name: "main.go"}, {Name: "pkg", Dir: true}, {Name: ".git"},
	})
	want := []string{dir + "/pkg", dir + "/main.go"}
	if !slices.Equal(kids, want) {
		t.Fatalf("kids = %q, want %q", kids, want)
	}
	if !dirs[dir+"/pkg"] || dirs[dir+"/main.go"] {
		t.Fatalf("dirs = %v", dirs)
	}
}
