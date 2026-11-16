// wsl.go is the distributions of the Windows Subsystem for Linux: a
// folder of one is read and written where it lives, by commands wsl.exe
// runs in the distribution, rather than through the share Windows mounts
// it on, which is slow and goes through another system's file system.

package remote

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"unicode/utf16"
)

// ErrNoWSL is the error of WSL here: only Windows runs a distribution.
var ErrNoWSL = errors.New("no WSL on this system")

// wslExe finds wsl.exe, once: the one Windows has, or else one on the
// path, which does not move while the app runs.
func wslExe() (string, error) {
	wslOnce.Do(func() {
		wslFound, wslErr = findWSLExe()
	})
	return wslFound, wslErr
}

var (
	wslOnce          sync.Once
	wslFound, wslErr = "", error(nil)
)

// findWSLExe finds wsl.exe: the one Windows has, or else one on the path.
func findWSLExe() (string, error) {
	if runtime.GOOS != "windows" {
		return "", ErrNoWSL
	}
	if root := os.Getenv("SystemRoot"); root != "" {
		p := filepath.Join(root, "System32", "wsl.exe")
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	p, err := exec.LookPath("wsl.exe")
	if err != nil {
		return "", fmt.Errorf("wsl.exe is not there: %w", ErrNoWSL)
	}
	return p, nil
}

// WSLSupported reports whether this system runs distributions of WSL.
func WSLSupported() bool {
	_, err := wslExe()
	return err == nil
}

// wslArgs are the arguments of wsl.exe that run command in distro,
// without its default shell: each is a word of its own, as wsl.exe joins
// them into the command line of the program.
func wslArgs(distro string, command ...string) []string {
	args := []string{"-d", distro, "-e"}
	return append(args, command...)
}

// wslTerminal is the command of a terminal in dir of a distribution,
// which runs the user's shell there, in that folder.
func wslTerminal(exe, distro, dir string) []string {
	args := []string{"-d", distro}
	if dir != "" {
		args = append(args, "--cd", dir)
	}
	return append([]string{exe}, args...)
}

// wslLauncher is the command of a script of ours in a distribution, given
// to sh as its -c argument: sh reads the length of the script, then the
// script itself, from its input, runs it, and leaves the rest of the input
// to it.
//
// The script goes to a file that sh runs, rather than through eval, so that
// the launcher holds no quotes at all: wsl.exe hands the arguments of its
// command line to the distribution, and what it makes of a quote in one is
// not the same in every version. A shell that reads a script from its input
// cannot be the launcher's sh either: dash, which is sh on Ubuntu, reads
// all of it into a buffer of its own as it parses it, so that a launcher
// read from the input finds the script gone, which is why the launcher comes
// as an argument.
const wslLauncher = `IFS= read -r n; f=$(mktemp); dd bs=1 count=$n of=$f 2>/dev/null; . $f; s=$?; rm -f $f; exit $s`

// wslScriptArgs are the arguments that run a script of ours in distro: sh
// runs the launcher, which reads the script from its input, as the launcher
// of a host of ssh does.
func wslScriptArgs(distro string) []string {
	return wslArgs(distro, "sh", "-c", wslLauncher)
}

// wslCheck checks that distro answers, with what wsl.exe says of it when
// it does not.
func wslCheck(ctx context.Context, exe, distro string) error {
	if distro == "" {
		return errors.New("no distribution of WSL to run")
	}
	cmd := exec.CommandContext(ctx, exe, wslArgs(distro, "true")...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return wslError(stderr.String(), err)
	}
	return nil
}

// wslGone reports whether what wsl.exe said is that the distribution is
// not there at all, rather than that a command in it failed.
func wslGone(stderr string) bool {
	msg := strings.ToLower(wslText(stderr))
	for _, gone := range []string{
		"no distribution with the supplied name",
		"wsl_e_distro_not_found",
		"has no installed distributions",
		"cannot find the path specified",
		"the system cannot find the file",
	} {
		if strings.Contains(msg, gone) {
			return true
		}
	}
	return false
}

// wslError is what to say of wsl.exe that failed: its own words, which
// come as UTF-16.
func wslError(stderr string, fallback error) error {
	msg := strings.TrimSpace(wslText(stderr))
	if i := strings.LastIndexByte(msg, '\n'); i >= 0 {
		msg = strings.TrimSpace(msg[i+1:])
	}
	if msg == "" {
		return fallback
	}
	return errors.New(msg)
}

// wslText is what wsl.exe wrote, in UTF-8: it writes UTF-16 where the
// output is not a terminal.
func wslText(s string) string {
	if !strings.ContainsRune(s, 0) {
		return s
	}
	b := []byte(s)
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return strings.TrimPrefix(string(utf16.Decode(u)), "\ufeff")
}

// WSLDistros returns the distributions wsl.exe has, the default first, as
// its list names them.
func WSLDistros() ([]string, error) {
	exe, err := wslExe()
	if err != nil {
		return nil, err
	}
	names, def, err := wslList(exe)
	if err != nil || len(names) == 0 {
		return nil, err
	}
	// The default is asked for as well, for a list that does not mark it.
	if def == "" {
		def, _ = wslDefault(exe)
	}
	if i := slices.Index(names, def); i > 0 {
		// The default is listed first, as wsl.exe lists its own.
		names = slices.Insert(slices.Delete(slices.Clone(names), i, i+1), 0, def)
	}
	return names, nil
}

// wslDefault returns the distribution wsl.exe starts when it is not told
// which, "" for none.
func wslDefault(exe string) (string, error) {
	out, err := wslOutput(exe, "--list", "--quiet")
	if err != nil {
		return "", err
	}
	for name := range strings.SplitSeq(out, "\n") {
		if name = strings.TrimSpace(name); name != "" {
			return name, nil
		}
	}
	return "", nil
}

// wslList returns the names of the distributions of wsl.exe, and the
// default, which its verbose list marks with a star and lists first.
func wslList(exe string) (names []string, def string, err error) {
	out, err := wslOutput(exe, "--list", "--verbose")
	if err != nil {
		return nil, "", err
	}
	names, def = parseWSLList(out)
	return names, def, nil
}

// parseWSLList reads the names of the distributions of wsl.exe's verbose
// list, and the default, which it marks with a star.
func parseWSLList(out string) (names []string, def string) {
	lines := strings.Split(out, "\n")
	if len(lines) < 1 {
		return nil, ""
	}
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		isDefault := strings.HasPrefix(strings.TrimSpace(line), "*")
		fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "*"))
		if len(fields) == 0 {
			continue
		}
		// The name may hold spaces: the state and the version follow it.
		name := fields[0]
		if len(fields) > 2 {
			name = strings.Join(fields[:len(fields)-2], " ")
		}
		names = append(names, name)
		if isDefault && def == "" {
			def = name
		}
	}
	return names, def
}

// wslOutput runs wsl.exe with args and returns what it wrote, in UTF-8:
// WSL_UTF8 asks the recent versions for it, and the older ones are read
// as the UTF-16 they wrote.
func wslOutput(exe string, args ...string) (string, error) {
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "WSL_UTF8=1")
	var out, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return "", wslError(stderr.String(), err)
	}
	return wslText(out.String()), nil
}

// WSLFolderOfWindows returns the path of a folder of this machine inside
// the distribution that reads it: /mnt/c/Users for C:\Users, with the
// distribution "" as any distribution reads that drive, and the path of a
// distribution itself for one under \\wsl.localhost or \\wsl$, with the
// distribution it is in. ok is false for a path no distribution has, as
// one of this machine.
func WSLFolderOfWindows(p string) (distro, rpath string, ok bool) {
	s := strings.ReplaceAll(p, `\`, "/")
	if distro, rpath, ok := wslShare(s); ok {
		return distro, rpath, true
	}
	if len(s) < 2 || s[1] != ':' || !isDriveLetter(s[0]) {
		return "", "", false
	}
	drive, rest := strings.ToLower(s[:1]), s[2:]
	if len(rest) == 0 || rest[0] != '/' {
		rest = "/" + rest
	}
	return "", "/mnt/" + drive + path.Clean(rest), true
}

// isDriveLetter reports whether c is the letter that names a drive of
// Windows.
func isDriveLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// WSLWindowsPath returns the path Windows reaches a path of a distribution
// by: the share it mounts the distribution on, \\wsl.localhost\<distro>\...,
// which Explorer shows and a dialog of Windows opens. ok is false for a path
// of this machine, and for one of a host of ssh, which Windows has no share
// of.
func WSLWindowsPath(p string) (string, bool) {
	key, rpath, ok := Split(p)
	if !ok {
		return "", false
	}
	scheme, name := SplitKey(key)
	if scheme != WSL {
		return "", false
	}
	unc := `\\wsl.localhost\` + name
	if rpath != "/" {
		unc += strings.ReplaceAll(rpath, "/", `\`)
	}
	return unc, true
}

// wslShare parts a path under \\wsl.localhost\<distro> or \\wsl$\<distro>
// into the distribution and the path inside it.
func wslShare(p string) (distro, rpath string, ok bool) {
	s := strings.ReplaceAll(p, `\`, "/")
	lower := strings.ToLower(s)
	for _, share := range []string{"//wsl.localhost/", "//wsl$/"} {
		rest, found := strings.CutPrefix(lower, share)
		if !found {
			continue
		}
		name, rp, _ := strings.Cut(rest, "/")
		if name == "" {
			return "", "", false
		}
		// The name keeps its case as the path spelled it.
		distro = s[len(share) : len(share)+len(name)]
		return distro, path.Clean("/" + rp), true
	}
	return "", "", false
}
