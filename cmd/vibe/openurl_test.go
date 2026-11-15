package main

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"vibe/internal/cli"
)

func TestOpenURLTargetQuery(t *testing.T) {
	file := filepath.Join(t.TempDir(), "src", "App.tsx")
	raw := "vibe://open?file=" + url.QueryEscape(file) + "&line=12&column=5"
	want := cli.Target{Path: filepath.Clean(file), Line: 12, Col: 5}
	if got, ok := openURLTarget(raw); !ok || got != want {
		t.Fatalf("openURLTarget(%q) = %+v, %v; want %+v", raw, got, ok, want)
	}
	raw = "vibe://open?file=" + url.QueryEscape(file)
	want = cli.Target{Path: filepath.Clean(file)}
	if got, ok := openURLTarget(raw); !ok || got != want {
		t.Fatalf("openURLTarget(%q) = %+v, %v; want %+v", raw, got, ok, want)
	}
}

func TestOpenURLTargetFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a Windows path is tested by TestOpenURLTargetFileWindows")
	}
	file := filepath.Join(t.TempDir(), "My App", "src", "App.tsx")
	escaped := strings.ReplaceAll(filepath.ToSlash(file), " ", "%20")
	for _, raw := range []string{
		"vibe://file" + escaped + ":12:5",
		"vibe://file/" + escaped + ":12:5",
	} {
		want := cli.Target{Path: filepath.Clean(file), Line: 12, Col: 5}
		if got, ok := openURLTarget(raw); !ok || got != want {
			t.Fatalf("openURLTarget(%q) = %+v, %v; want %+v", raw, got, ok, want)
		}
	}
}

func TestOpenURLTargetFileUnescaped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a Windows path is tested by TestOpenURLTargetFileWindows")
	}
	dir := filepath.ToSlash(t.TempDir())
	for _, name := range []string{"a#b.tsx", "a?b.tsx", "100%.tsx", "[slug].tsx"} {
		raw := "vibe://file" + dir + "/" + name + ":3:1"
		want := cli.Target{Path: filepath.Join(dir, name), Line: 3, Col: 1}
		if got, ok := openURLTarget(raw); !ok || got != want {
			t.Fatalf("openURLTarget(%q) = %+v, %v; want %+v", raw, got, ok, want)
		}
	}
}

func TestIsVibeURL(t *testing.T) {
	for arg, want := range map[string]bool{
		"vibe://file/a.go:1:2": true,
		"VIBE://open?file=/a":  true,
		"/Users/me/vibe":       false,
		`C:\Users\me`:          false,
		"vibe.go":              false,
	} {
		if got := isVibeURL(arg); got != want {
			t.Errorf("isVibeURL(%q) = %v, want %v", arg, got, want)
		}
	}
}

func TestOpenURLTargetFileWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("a Windows path")
	}
	file := filepath.Join(t.TempDir(), "My App", "src", "App.tsx")
	raw := "vibe://file/" + filepath.ToSlash(file) + ":12:5"
	want := cli.Target{Path: filepath.Clean(file), Line: 12, Col: 5}
	if got, ok := openURLTarget(raw); !ok || got != want {
		t.Fatalf("openURLTarget(%q) = %+v, %v; want %+v", raw, got, ok, want)
	}
}

func TestOpenURLTargetRejects(t *testing.T) {
	file := filepath.Join(t.TempDir(), "App.tsx")
	for _, raw := range []string{
		"https://example.com/App.tsx:1:2",
		"vibe://other?file=" + url.QueryEscape(file),
		"vibe://file",
		"vibe://files/" + filepath.ToSlash(file),
		"vibe://open?file=",
		"vibe://open?file=" + url.QueryEscape(file) + "&line=0",
		"vibe://open?file=" + url.QueryEscape(file) + "&column=-1",
		"vibe://open?file=" + url.QueryEscape(file) + "&line=x",
		"vibe://open?file=src/App.tsx",
	} {
		if got, ok := openURLTarget(raw); ok {
			t.Fatalf("openURLTarget(%q) = %+v, true; want false", raw, got)
		}
	}
}
