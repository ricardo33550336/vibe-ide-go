package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The vibe command runs the app's executable with its arguments, told it
// is the command.
func TestScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	app := filepath.Join(dir, "My Vibe.app")
	moved := filepath.Join(dir, "Moved.app")
	for _, a := range []string{app, moved} {
		exe := filepath.Join(a, "Contents", "MacOS", "Vibe")
		os.MkdirAll(filepath.Dir(exe), 0o755)
		os.WriteFile(exe, []byte("#!/bin/sh\necho \"$0\" \"$VIBE_CLI\"\nfor a in \"$@\"; do echo \"$a\"; done\n"), 0o755)
	}
	if err := write(bin, app); err != nil {
		t.Fatal(err)
	}
	// mdfind knows the app that moved.
	os.WriteFile(filepath.Join(bin, "mdfind"), []byte("#!/bin/sh\necho '"+moved+"'\n"), 0o755)
	run := func(args ...string) []string {
		cmd := exec.Command(filepath.Join(bin, "vibe"), args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin")
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	}
	exe := filepath.Join(app, "Contents", "MacOS", "Vibe")
	if got := run(); !slices.Equal(got, []string{exe + " 1"}) {
		t.Errorf("no args: %v", got)
	}
	got := run("-g", "a b.go:3", "run")
	if want := []string{exe + " 1", "-g", "a b.go:3", "run"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	os.RemoveAll(app)
	got = run("x")
	if want := []string{filepath.Join(moved, "Contents", "MacOS", "Vibe") + " 1", "x"}; !slices.Equal(got, want) {
		t.Errorf("moved: got %v, want %v", got, want)
	}
}

// The Windows command is a batch file that runs the app's executable,
// preserving its arguments and exit status.
func TestWindowsScript(t *testing.T) {
	if os.Getenv("VIBE_TEST_HELPER") != "" {
		fmt.Println(os.Getenv("VIBE_CLI"))
		for _, arg := range os.Args[1:] {
			fmt.Printf("%q\n", arg)
		}
		os.Exit(7)
	}
	if runtime.GOOS != "windows" {
		t.Skip("a Windows batch file")
	}
	comspec := os.Getenv("COMSPEC")
	if comspec == "" {
		comspec = `C:\Windows\System32\cmd.exe`
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	app := filepath.Join(dir, "My 100% Vibe", "Vibe.exe")
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(app), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app, data, 0o755); err != nil {
		t.Fatal(err)
	}
	localData := filepath.Join(dir, "Local")
	t.Setenv("LOCALAPPDATA", localData)
	bin := filepath.Join(localData, "Vibe", "bin")
	t.Setenv("PATH", t.TempDir())
	t.Setenv("SHELL", filepath.Join(dir, "missing"))
	p, err := Install(Dirs(dir, LoginPath()), app)
	if err != nil || p != filepath.Join(bin, "vibe.cmd") {
		t.Fatalf("installed at %q: %v", p, err)
	}
	if _, err := Install([]string{bin}, app); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	args := []string{"/d", "/c", p, "-test.run=TestWindowsScript", "--", "-g", "a b.go:3"}
	cmd := exec.Command(comspec, args...)
	cmd.Env = append(os.Environ(), "VIBE_TEST_HELPER=1")
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 7 {
		t.Fatalf("command exit: %v\n%s", err, out)
	}
	got := strings.ReplaceAll(string(out), "\r\n", "\n")
	want := "1\n\"-test.run=TestWindowsScript\"\n\"--\"\n\"-g\"\n\"a b.go:3\"\n"
	if got != want {
		t.Fatalf("command output:\n%q\nwant:\n%q", got, want)
	}
	if err := os.Remove(app); err != nil {
		t.Fatal(err)
	}
	out, err = exec.Command(comspec, "/d", "/c", p).CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 || !strings.Contains(string(out), "Vibe.exe is not found") {
		t.Fatalf("missing app exit: %v\n%s", err, out)
	}
	removed, err := Uninstall([]string{bin})
	if err != nil || !slices.Equal(removed, []string{p}) {
		t.Fatalf("removed %v: %v", removed, err)
	}
}

// Refresh brings an older Vibe's command up to date, and leaves another
// program's.
func TestRefresh(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	dir := t.TempDir()
	old, foreign := filepath.Join(dir, "old"), filepath.Join(dir, "foreign")
	os.MkdirAll(old, 0o755)
	os.MkdirAll(foreign, 0o755)
	os.WriteFile(filepath.Join(old, "vibe"), []byte("#!/bin/sh\n# "+scriptMark+"\nopen -b x \"$@\"\n"), 0o755)
	os.WriteFile(filepath.Join(foreign, "vibe"), []byte("#!/bin/sh\necho other\n"), 0o755)
	Refresh([]string{old, foreign, filepath.Join(dir, "none")}, "/Apps/Vibe.app")
	if data, _ := os.ReadFile(filepath.Join(old, "vibe")); string(data) != script("/Apps/Vibe.app") {
		t.Errorf("not refreshed: %s", data)
	}
	if data, _ := os.ReadFile(filepath.Join(foreign, "vibe")); string(data) != "#!/bin/sh\necho other\n" {
		t.Errorf("another program's changed: %s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "none", "vibe")); err == nil {
		t.Error("installed where there was none")
	}
}

func TestInstallAndUninstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first"), filepath.Join(dir, "second")
	p, err := Install([]string{first, second}, "")
	if err != nil || p != filepath.Join(first, "vibe") {
		t.Fatalf("installed at %q: %v", p, err)
	}
	if info, err := os.Stat(p); err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("not executable: %v", err)
	}
	// Again, over its own.
	if _, err := Install([]string{first}, ""); err != nil {
		t.Fatal(err)
	}
	// Another program's command stays.
	os.MkdirAll(second, 0o755)
	foreign := filepath.Join(second, "vibe")
	os.WriteFile(foreign, []byte("#!/bin/sh\necho other\n"), 0o755)
	if _, err := Install([]string{second}, ""); err == nil {
		t.Error("overwrote another program's vibe")
	}
	removed, err := Uninstall([]string{first, second})
	if err != nil || !slices.Equal(removed, []string{p}) {
		t.Fatalf("removed %v: %v", removed, err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Error("removed another program's vibe")
	}
}

func TestDirs(t *testing.T) {
	home := t.TempDir()
	local := filepath.Join(home, ".local", "bin")
	bin := filepath.Join(home, "bin")
	custom := filepath.Join(home, ".nvm", "versions", "node", "bin")
	if runtime.GOOS == "windows" {
		localData := filepath.Join(home, "Local")
		t.Setenv("LOCALAPPDATA", localData)
		own := filepath.Join(localData, "Vibe", "bin")
		if got := Dirs(home, []string{custom}); !slices.Equal(got, []string{own}) {
			t.Fatalf("Windows installation uses another tool's folder: %v", got)
		}
		if got := InstalledDirs(home, []string{custom}); !slices.Equal(got, []string{custom, own}) {
			t.Fatalf("Windows management destinations: %v", got)
		}
		return
	}
	for _, tt := range []struct {
		name       string
		path, want []string
	}{
		{"prefer user folders", []string{"/usr/local/bin", bin, local, local}, []string{local, bin, "/usr/local/bin"}},
		{"tool folders", []string{custom, "/opt/homebrew/bin", "/Applications/Other.app/Contents/bin", local}, []string{local}},
		{"relative paths", []string{".", "bin", ""}, nil},
		{"empty PATH", nil, nil},
		{"fish-style output", []string{local + " /usr/bin /bin"}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Dirs(home, tt.path); !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
	dirs := InstalledDirs(home, []string{custom})
	if want := []string{custom, local, bin, "/usr/local/bin"}; !slices.Equal(dirs, want) {
		t.Fatalf("management destinations: %v, want %v", dirs, want)
	}
}

func TestInstallReachableCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	home := t.TempDir()
	if _, err := Install(Dirs(home, []string{"/usr/bin", "/bin", "/opt/homebrew/bin"}), ""); err == nil || !strings.Contains(err.Error(), "PATH") {
		t.Fatalf("missing PATH guidance: %v", err)
	}
	local := filepath.Join(home, ".local", "bin")
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := Install(Dirs(home, []string{local, bin}), "/Applications/Vibe.app")
	if err != nil || p != filepath.Join(bin, "vibe") {
		t.Fatalf("user installation: %q, %v", p, err)
	}
	Refresh([]string{bin}, "/New/Vibe.app")
	if data, _ := os.ReadFile(p); string(data) != script("/New/Vibe.app") {
		t.Fatalf("command was not refreshed: %s", data)
	}
	removed, err := Uninstall([]string{bin})
	if err != nil || !slices.Contains(removed, p) {
		t.Fatalf("command was not removed: %v, %v", removed, err)
	}
}

func TestInstallReusesExistingCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	home := t.TempDir()
	local := filepath.Join(home, ".local", "bin")
	bin := filepath.Join(home, "bin")
	if err := write(bin, "/Old/Vibe.app"); err != nil {
		t.Fatal(err)
	}
	p, err := Install(Dirs(home, []string{local, bin}), "/New/Vibe.app")
	if err != nil || p != filepath.Join(bin, "vibe") {
		t.Fatalf("reinstalled at %q: %v", p, err)
	}
	if data, _ := os.ReadFile(p); string(data) != script("/New/Vibe.app") {
		t.Fatalf("existing command not updated: %s", data)
	}
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Fatalf("created another installation directory: %v", err)
	}
}

func TestInstallSkipsUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("Unix directory permissions")
	}
	dir := t.TempDir()
	blocked, writable := filepath.Join(dir, "blocked"), filepath.Join(dir, "writable")
	if err := os.Mkdir(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0o755) })
	if p, err := Install([]string{blocked, writable}, ""); err != nil || p != filepath.Join(writable, "vibe") {
		t.Fatalf("unreadable directory blocked installation: %q, %v", p, err)
	}
	// A readable, unwritable folder must give useful guidance too.
	if err := os.Chmod(blocked, 0o555); err != nil {
		t.Fatal(err)
	}
	if _, err := Install([]string{blocked}, ""); err == nil || !strings.Contains(err.Error(), "add ~/.local/bin") {
		t.Fatalf("missing failure guidance: %v", err)
	}
}

func TestPreserveHomebrewCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "brew-wrapper")
	data := []byte("#!/bin/sh\n# Managed by Homebrew; use brew uninstall --cask vibe to remove it.\n")
	if err := os.WriteFile(wrapper, data, 0o755); err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(dir, "vibe")
	if err := os.Symlink(wrapper, command); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	if _, err := Install([]string{empty, dir}, ""); err == nil || !strings.Contains(err.Error(), "another program") {
		t.Fatalf("expected ownership protection: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(empty, "vibe")); !os.IsNotExist(err) {
		t.Fatalf("shadowed Homebrew's command: %v", err)
	}
	Refresh([]string{dir}, "/Applications/Vibe.app")
	if removed, err := Uninstall([]string{dir}); err != nil || len(removed) != 0 {
		t.Fatalf("removed Homebrew command: %v, %v", removed, err)
	}
	if target, err := os.Readlink(command); err != nil || target != wrapper {
		t.Fatalf("changed Homebrew symlink: %q, %v", target, err)
	}
	if got, _ := os.ReadFile(wrapper); !slices.Equal(got, data) {
		t.Fatalf("changed Homebrew wrapper: %s", got)
	}
}

func TestPreserveBrokenCommandLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "vibe")
	target := filepath.Join(dir, "missing")
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
	if _, err := Install([]string{dir}, ""); err == nil {
		t.Fatal("replaced an unowned broken symlink")
	}
	if got, err := os.Readlink(p); err != nil || got != target {
		t.Fatalf("changed broken symlink: %q, %v", got, err)
	}
}

// Opened from the Finder, the app has the system's folders only: the login
// shell's come first, and none twice.
func TestShellPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	tools := t.TempDir()
	sh := filepath.Join(t.TempDir(), "zsh")
	script := "#!/bin/sh\nPATH=" + tools + ":/usr/bin:$PATH; export PATH; shift; exec /bin/sh -c \"$1\"\n"
	if err := os.WriteFile(sh, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", sh)
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")
	want := []string{tools, "/usr/bin", "/bin", "/usr/sbin", "/sbin"}
	if got := ShellPath(); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoginPathExcludesFinderFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	dir := t.TempDir()
	sh := filepath.Join(dir, "shell")
	if err := os.WriteFile(sh, []byte("#!/bin/sh\necho VIBE_PATH=/usr/bin:/bin\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", sh)
	t.Setenv("PATH", "/usr/local/bin:/usr/bin:/bin")
	if got := Dirs(dir, LoginPath()); len(got) != 0 {
		t.Fatalf("installation chose a Finder-only directory: %v", got)
	}
	if got := ShellPath(); !slices.Contains(got, "/usr/local/bin") {
		t.Fatalf("application lost its fallback PATH: %v", got)
	}
	t.Setenv("SHELL", filepath.Join(dir, "missing"))
	if got := LoginPath(); len(got) != 0 {
		t.Fatalf("failed shell produced installation candidates: %v", got)
	}
}

func TestLoginPathKeepsOutputOnFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	sh := filepath.Join(t.TempDir(), "shell")
	if err := os.WriteFile(sh, []byte("#!/bin/sh\necho VIBE_PATH=/usr/bin:/bin\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", sh)
	if got := LoginPath(); !slices.Equal(got, []string{"/usr/bin", "/bin"}) {
		t.Fatalf("discarded usable output: %v", got)
	}
}

func TestLoginPathFish(t *testing.T) {
	fish, err := exec.LookPath("fish")
	if err != nil {
		t.Skip("fish is not installed")
	}
	sh := filepath.Join(t.TempDir(), "shell")
	// Suppress the user's fish configuration while running the actual query.
	if err := os.WriteFile(sh, []byte("#!/bin/sh\nexec "+shellQuote(fish)+" --no-config -c \"$2\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(t.TempDir(), "Home with spaces")
	local := filepath.Join(home, ".local", "bin")
	t.Setenv("SHELL", sh)
	t.Setenv("PATH", local+":/usr/bin:/bin")
	if got := LoginPath(); !slices.Equal(got, []string{local, "/usr/bin", "/bin"}) {
		t.Fatalf("fish PATH lost directory boundaries: %q", got)
	}
	p, err := Install(Dirs(home, LoginPath()), "/Applications/Vibe.app")
	if err != nil || p != filepath.Join(local, "vibe") {
		t.Fatalf("fish installation: %q, %v", p, err)
	}
}
