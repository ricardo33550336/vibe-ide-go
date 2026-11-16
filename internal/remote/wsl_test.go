package remote

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestWSLPaths(t *testing.T) {
	key := Key(WSL, "Ubuntu-22.04")
	if key != "wsl:Ubuntu-22.04" {
		t.Fatalf("Key = %q", key)
	}
	p := Path(key, "/home/me/proj/")
	if p != "/wsl:Ubuntu-22.04/home/me/proj" {
		t.Fatalf("Path = %q", p)
	}
	gotKey, rpath, ok := Split(filepath.Join(p, "a", "..", "b.go"))
	if !ok || gotKey != key || rpath != "/home/me/proj/b.go" {
		t.Fatalf("Split = %q %q %v", gotKey, rpath, ok)
	}
	if root := Path(key, "/"); root != "/wsl:Ubuntu-22.04" {
		t.Fatalf("the root of a distribution is %q", root)
	}
	if _, rpath, _ := Split("/wsl:Ubuntu-22.04"); rpath != "/" {
		t.Fatalf("the root splits to %q", rpath)
	}
	// A path of a host of ssh keeps the spelling it had before WSL.
	if got := Path(Key(SSH, "me@box:2222"), "/srv"); got != "/ssh:me@box:2222/srv" {
		t.Fatalf("Path of a host = %q", got)
	}
	if got := Display(p); got != "/home/me/proj" {
		t.Errorf("Display = %q", got)
	}
	if got := URL(p); got != "" {
		t.Errorf("URL of a distribution = %q", got)
	}
	if got := NameOf(p); got != "Ubuntu-22.04" {
		t.Errorf("NameOf = %q", got)
	}
	if got := SchemeOf(p); got != WSL {
		t.Errorf("SchemeOf = %q", got)
	}
	if got := SchemeOf("/ssh:box/srv"); got != SSH {
		t.Errorf("SchemeOf of a host = %q", got)
	}
	if got := On(p, "/home/me/x"); got != "/wsl:Ubuntu-22.04/home/me/x" {
		t.Errorf("On = %q", got)
	}
	for _, local := range []string{"/wsl:", "/wsl:/x", "/wsl:Ubuntu:2/x", "/mnt/c/Users/me"} {
		if Is(local) {
			t.Errorf("%q is taken for remote", local)
		}
	}
}

func TestWSLFolderOfWindows(t *testing.T) {
	tests := []struct {
		in, distro, path string
		ok               bool
	}{
		{`C:\Users\me\proj`, "", "/mnt/c/Users/me/proj", true},
		{`c:\Users\me`, "", "/mnt/c/Users/me", true},
		{`D:\`, "", "/mnt/d/", true},
		{`\\wsl.localhost\Ubuntu\home\me\proj`, "Ubuntu", "/home/me/proj", true},
		{`\\WSL$\Debian\srv\app`, "Debian", "/srv/app", true},
		{`\\wsl.localhost\Ubuntu`, "Ubuntu", "/", true},
		{`/Users/me`, "", "", false},
		{`relative\x`, "", "", false},
		{`\\server\share\x`, "", "", false},
	}
	for _, tt := range tests {
		distro, p, ok := WSLFolderOfWindows(tt.in)
		if ok != tt.ok || distro != tt.distro || p != tt.path {
			t.Errorf("WSLFolderOfWindows(%q) = %q %q %v, want %q %q %v", tt.in, distro, p, ok, tt.distro, tt.path, tt.ok)
		}
	}
}

func TestWSLLists(t *testing.T) {
	out := "  NAME            STATE           VERSION\r\n* Ubuntu-22.04    Running         2\r\n  Debian          Stopped         2\r\n  My Distro  Stopped  2\r\n"
	names, def := parseWSLList(out)
	if want := []string{"Ubuntu-22.04", "Debian", "My Distro"}; !slices.Equal(names, want) {
		t.Fatalf("names = %q, want %q", names, want)
	}
	if def != "Ubuntu-22.04" {
		t.Fatalf("default = %q", def)
	}
	out = "  NAME  STATE  VERSION\n  Ubuntu  Running  2\n"
	if names, def := parseWSLList(out); !slices.Equal(names, []string{"Ubuntu"}) || def != "" {
		t.Fatalf("a list without a star = %q, %q", names, def)
	}
}

func TestWSLText(t *testing.T) {
	// wsl.exe writes UTF-16, little endian, where its output is not a
	// terminal.
	var b []byte
	for _, r := range "no distribution" {
		b = append(b, byte(r), 0)
	}
	if got := wslText(string(b)); got != "no distribution" {
		t.Fatalf("wslText = %q", got)
	}
	if got := wslText("plain UTF-8\n"); got != "plain UTF-8\n" {
		t.Fatalf("wslText of UTF-8 = %q", got)
	}
	b = append([]byte{0xff, 0xfe}, b...)
	if got := wslText(string(b)); got != "no distribution" {
		t.Fatalf("wslText of a mark = %q", got)
	}
}

func TestWSLGone(t *testing.T) {
	if !wslGone("There is no distribution with the supplied name.\r\n") {
		t.Error("a missing distribution is not seen as gone")
	}
	if !wslGone("WSL_E_DISTRO_NOT_FOUND") {
		t.Error("an old message is not seen as gone")
	}
	if wslGone("sh: 1: nope: not found") {
		t.Error("a command that failed is seen as the distribution gone")
	}
	if got := wslError("There is no distribution with the supplied name.\r\n", nil); got == nil || got.Error() != "There is no distribution with the supplied name." {
		t.Errorf("wslError = %v", got)
	}
}

func TestWSLCommands(t *testing.T) {
	if got := wslArgs("Ubuntu", "sh", "-s"); !slices.Equal(got, []string{"-d", "Ubuntu", "-e", "sh", "-s"}) {
		t.Errorf("wslArgs = %q", got)
	}
	if got := wslTerminal("wsl.exe", "Ubuntu", "/home/me/proj"); !slices.Equal(got, []string{"wsl.exe", "-d", "Ubuntu", "--cd", "/home/me/proj"}) {
		t.Errorf("wslTerminal = %q", got)
	}
}

func TestWSLCommand(t *testing.T) {
	c := For(Key(WSL, "Ubuntu"))
	cmd := c.command(context.Background(), "", nil, "printf '%s\n' hi", "/home/me")
	if got := cmd.Args; !slices.Equal(got, []string{"wsl.exe", "-d", "Ubuntu", "-e", "sh", "-s"}) {
		t.Fatalf("args = %q", got)
	}
	in, err := io.ReadAll(cmd.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	rest, ok := strings.CutPrefix(string(in), wslScriptInput)
	if !ok {
		t.Fatalf("the input does not start with the launcher: %q", in)
	}
	length, script, ok := strings.Cut(rest, "\n")
	if !ok || length != strconv.Itoa(len(script)) {
		t.Fatalf("the length is %q for a script of %d bytes", length, len(script))
	}
	// Nothing of the script is on the command line, which wsl.exe joins
	// without quoting.
	for _, s := range []string{"set -- '/home/me'", "printf '%s\n' hi"} {
		if !strings.Contains(script, s) {
			t.Errorf("the script has no %q: %q", s, script)
		}
	}
}

func TestWSLTerminalCommand(t *testing.T) {
	c := For(Key(WSL, "Ubuntu-22.04"))
	if got := c.TerminalCommand("/home/me/proj"); !slices.Equal(got, []string{"wsl.exe", "-d", "Ubuntu-22.04", "--cd", "/home/me/proj"}) {
		t.Fatalf("TerminalCommand = %q", got)
	}
}

// TestWSLScriptInput runs the launcher of a distribution's command in a
// POSIX shell, which is what sh there reads from wsl.exe: the script by
// its length, and the rest of the input left to it.
func TestWSLScriptInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a POSIX shell")
	}
	script := "set -- 'a b'\n" + preamble + "printf '%s|' \"$@\"; cat"
	in := wslScriptInput + strconv.Itoa(len(script)) + "\n" + script + "tail"
	cmd := exec.Command("sh", "-s")
	cmd.Stdin = strings.NewReader(in)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if got := out.String(); got != "a b|tail" {
		t.Fatalf("the script printed %q", got)
	}
}
