package remote

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"path/filepath"
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
	if got := cmd.Args; !slices.Equal(got, []string{"wsl.exe", "-d", "Ubuntu", "-e", "sh", "-c", wslLauncher}) {
		t.Fatalf("args = %q", got)
	}
	in, err := io.ReadAll(cmd.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	length, script, ok := strings.Cut(string(in), "\n")
	if !ok || length != strconv.Itoa(len(script)) {
		t.Fatalf("the length is %q for a script of %d bytes", length, len(script))
	}
	// Nothing of the script is on the command line, which wsl.exe hands over
	// as it is.
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

// TestWSLLauncher runs the launcher of a distribution's command in the
// shells of this machine, which read the script from its input and leave the
// rest of it to the script. dash is sh on Ubuntu, and reads a script from its
// input away from anything else that would, which is why the launcher comes
// as an argument and the script goes to a file.
func TestWSLLauncher(t *testing.T) {
	var shells []string
	for _, name := range []string{"sh", "dash", "bash"} {
		if p, err := exec.LookPath(name); err == nil {
			shells = append(shells, p)
		}
	}
	if len(shells) == 0 {
		t.Skip("no POSIX shell")
	}
	// The script and the input after it, which stays the script's own.
	script := "set -- 'a b'\n" + preamble + "printf '%s|' \"$@\"; cat\n"
	for _, sh := range shells {
		in := strconv.Itoa(len(script)) + "\n" + script + "tail"
		cmd := exec.Command(sh, "-c", wslLauncher)
		cmd.Stdin = strings.NewReader(in)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			t.Errorf("%s: %v: %s", sh, err, out.String())
			continue
		}
		if got := out.String(); got != "a b|tail" {
			t.Errorf("%s: the script printed %q, want %q", sh, got, "a b|tail")
		}
	}
}

// TestRemotePathsKeepSlashes keeps the paths of a machine the host's, as
// Windows would spell them with backslashes once a filepath function of it
// joined one: the tree and the open documents compare paths as strings, and
// a path of a machine spelled two ways names two files.
func TestRemotePathsKeepSlashes(t *testing.T) {
	key := Key(WSL, "Ubuntu")
	root := Path(key, "/home/pepe/proj")
	if root != "/wsl:Ubuntu/home/pepe/proj" {
		t.Fatalf("Path = %q", root)
	}
	tests := []struct{ got, want string }{
		{Join(root, "a.go"), "/wsl:Ubuntu/home/pepe/proj/a.go"},
		{Join(root, "a/../b.go"), "/wsl:Ubuntu/home/pepe/proj/b.go"},
		// A path of a machine that Windows spelled with backslashes is
		// joined back into the host's spelling.
		{Join(`\wsl:Ubuntu\home\pepe\proj`, "a.go"), "/wsl:Ubuntu/home/pepe/proj/a.go"},
		{Dir("/wsl:Ubuntu/home/pepe/proj/a.go"), root},
		{Dir(root), "/wsl:Ubuntu/home/pepe"},
		{Dir("/wsl:Ubuntu/home"), "/wsl:Ubuntu"},
		{Base("/wsl:Ubuntu/home/pepe/proj/a.go"), "a.go"},
		{Clean(`\wsl:Ubuntu\home\pepe\proj\`), root},
		{Clean(root + "/./x/../y.go"), root + "/y.go"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
	for _, tt := range []struct {
		path, dir string
		in        bool
	}{
		{root + "/a.go", root, true},
		{root, root, true},
		{root + "x/a.go", root, false},
		{Path(Key(WSL, "Debian"), "/home/pepe/proj/a.go"), root, false},
	} {
		if got := In(tt.path, tt.dir); got != tt.in {
			t.Errorf("In(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.in)
		}
	}
	// A path of this machine is the filepath package's, as it was.
	if got := Join("/tmp/x", "a.go"); got != filepath.Join("/tmp/x", "a.go") {
		t.Errorf("Join of this machine = %q", got)
	}
	if got := In(filepath.Join("/tmp/x", "a.go"), "/tmp/x"); !got {
		t.Error("In of this machine is false")
	}
}

// TestWSLWindowsPath keeps how Windows reaches a folder of a distribution:
// through the share it mounts it on, which Explorer opens.
func TestWSLWindowsPath(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"/wsl:Ubuntu/home/pepe/proj", `\\wsl.localhost\Ubuntu\home\pepe\proj`, true},
		{"/wsl:Ubuntu/home/pepe/proj/main.go", `\\wsl.localhost\Ubuntu\home\pepe\proj\main.go`, true},
		{"/wsl:Ubuntu", `\\wsl.localhost\Ubuntu`, true},
		{"/wsl:My Distro/srv", `\\wsl.localhost\My Distro\srv`, true},
		// A host of ssh has no share of this machine, nor has one of them.
		{"/ssh:box/srv/app", "", false},
		{"/tmp/x", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := WSLWindowsPath(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("WSLWindowsPath(%q) = %q %v, want %q %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}
