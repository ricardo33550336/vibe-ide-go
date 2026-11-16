// Package remote reaches folders on other machines, with nothing
// installed on the other side: over SSH, with the system's ssh command,
// one connection to each host which every command shares, and, on
// Windows, in a distribution of the Windows Subsystem for Linux, with
// wsl.exe. The files are read and written by small shell scripts run
// there, so that a folder of a distribution is read where it lives,
// rather than through the slow share Windows mounts it on.
//
// The editor knows a remote file by a path of its own, /ssh:host/path or
// /wsl:distro/path, which the path functions of Go take as any other: the
// folder of a file, its name and paths relative to a folder come out as
// they should.
package remote

import (
	"errors"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// The schemes of remote machines: the hosts of ssh, and the distributions
// of the Windows Subsystem for Linux.
const (
	SSH = "ssh"
	WSL = "wsl"
)

// Key names the machine of a scheme: the key of a host of ssh is its name
// alone, so that the folders kept of one still name it, and that of a
// distribution is its name after "wsl:".
func Key(scheme, name string) string {
	if scheme == SSH || scheme == "" {
		return name
	}
	return scheme + ":" + name
}

// SplitKey parts a key into its scheme and the name of the machine, which
// is ssh's for a key without a scheme.
func SplitKey(key string) (scheme, name string) {
	if rest, ok := strings.CutPrefix(key, WSL+":"); ok && rest != "" && !strings.ContainsAny(rest, "/\\") {
		return WSL, rest
	}
	return SSH, key
}

// Path returns the path the editor knows the file at p on the machine of
// key by.
func Path(key, p string) string {
	scheme, name := SplitKey(key)
	root := "/" + scheme + ":" + name
	p = path.Clean("/" + p)
	if p == "/" {
		return root
	}
	return root + p
}

// Split returns the key of the machine of a remote path and the path on
// that machine, false for a path of this machine.
func Split(p string) (key, rpath string, ok bool) {
	// Windows spells a path of a machine with backslashes once a filepath
	// function of it joined one, and a machine's path is the host's: with
	// slashes, on every system.
	s := strings.ReplaceAll(p, `\`, "/")
	for _, scheme := range []string{SSH, WSL} {
		rest, ok := strings.CutPrefix(s, "/"+scheme+":")
		if !ok {
			continue
		}
		name, rp, _ := strings.Cut(rest, "/")
		if name == "" || scheme == WSL && strings.Contains(name, ":") {
			return "", "", false
		}
		return Key(scheme, name), path.Clean("/" + rp), true
	}
	return "", "", false
}

// Clean returns p as the editor spells it: a path of a machine with
// slashes, which Windows makes of its own of the paths the filepath
// package joins, and one of this machine as filepath.Clean has it.
func Clean(p string) string {
	if key, rpath, ok := Split(p); ok {
		return Path(key, rpath)
	}
	return filepath.Clean(p)
}

// Join joins a folder and the name of a file in it, as Clean spells the
// result: with slashes in a folder of a machine, whose paths are the
// host's, and as the system has them in one of this machine.
func Join(dir, name string) string {
	if key, rpath, ok := Split(dir); ok {
		return Path(key, path.Join(rpath, filepath.ToSlash(name)))
	}
	return filepath.Join(dir, name)
}

// Dir returns the folder of a file, as Clean spells it.
func Dir(p string) string {
	if key, rpath, ok := Split(p); ok {
		return Path(key, path.Dir(rpath))
	}
	return filepath.Dir(p)
}

// Base returns the name of a file, without its folder.
func Base(p string) string {
	if _, rpath, ok := Split(p); ok {
		return path.Base(rpath)
	}
	return filepath.Base(p)
}

// In reports whether path is dir, or a file in it, as Clean spells both.
func In(path, dir string) bool {
	d := strings.TrimSuffix(Clean(dir), "/")
	if d == "" {
		return false
	}
	p := Clean(path)
	return p == d || strings.HasPrefix(p, d+"/") || strings.HasPrefix(p, d+`\`)
}

// Is reports whether p is the path of a remote file.
func Is(p string) bool {
	_, _, ok := Split(p)
	return ok
}

// IsAbs reports whether p names a file from a root, as filepath.IsAbs
// says of a path of this machine, or as a path of a machine does, which
// Windows does not know to be absolute.
func IsAbs(p string) bool { return Is(p) || filepath.IsAbs(p) }

// HostOf returns the key of the machine of a remote path, "" for a path
// of this machine.
func HostOf(p string) string {
	key, _, _ := Split(p)
	return key
}

// NameOf returns how the machine of a remote path shows: the name of the
// host, or of the distribution. It is "" for a path of this machine.
func NameOf(p string) string {
	_, name := SplitKey(HostOf(p))
	return name
}

// SchemeOf returns the scheme of the machine of a remote path, "" for a
// path of this machine.
func SchemeOf(p string) string {
	scheme, _ := SplitKey(HostOf(p))
	if HostOf(p) == "" {
		return ""
	}
	return scheme
}

// On returns p, a path a program on the machine of ref wrote, as the
// editor knows it: as ref's machine has it when ref is remote.
func On(ref, p string) string {
	if key, _, ok := Split(ref); ok && strings.HasPrefix(p, "/") && !Is(p) {
		return Path(key, p)
	}
	return p
}

// Display is how a remote path shows, as ssh's scp-like syntax spells it:
// host:/path for a host of ssh, and the path itself in a distribution,
// which is a path of that Linux as it is.
func Display(p string) string {
	key, rpath, ok := Split(p)
	if !ok {
		return p
	}
	scheme, name := SplitKey(key)
	if scheme == WSL {
		return rpath
	}
	return name + ":" + rpath
}

// URL returns the ssh:// URL of a remote path, "" for a path of this
// machine and for one of a distribution, which ssh cannot reach.
func URL(p string) string {
	key, rpath, ok := Split(p)
	if !ok {
		return ""
	}
	scheme, name := SplitKey(key)
	if scheme != SSH {
		return ""
	}
	return "ssh://" + name + rpath
}

// ParseURL returns the path of the folder an ssh://[user@]host[:port]/path
// URL names.
func ParseURL(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != SSH || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("use ssh://[user@]host[:port]/path")
	}
	if _, ok := u.User.Password(); ok {
		return "", errors.New("passwords go in the SSH agent or ~/.ssh/config, not in the URL")
	}
	host := u.Host
	if u.User != nil {
		host = u.User.Username() + "@" + host
	}
	if err := CheckHost(host); err != nil {
		return "", err
	}
	return Path(Key(SSH, host), u.Path), nil
}

// CheckHost returns why host cannot name a machine to connect to: a name
// of ~/.ssh/config, or [user@]host[:port].
func CheckHost(host string) error {
	switch {
	case host == "":
		return errors.New("enter a host")
	case strings.HasPrefix(host, "-"):
		return errors.New("a host cannot start with -")
	case strings.ContainsAny(host, "/\\\x00\r\n\t '\"`$;&|<>(){}*?!#"):
		return errors.New("a host has no spaces, slashes or quotes")
	}
	if _, port := splitPort(host); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return errors.New("the port is a number from 1 to 65535")
		}
	}
	return nil
}

// splitPort parts [user@]host[:port] at its port, "" for none. A host of
// IPv6 is in brackets, as [::1]:22.
func splitPort(host string) (string, string) {
	if i := strings.LastIndexByte(host, ']'); i >= 0 {
		if rest, ok := strings.CutPrefix(host[i+1:], ":"); ok {
			return host[:i+1], rest
		}
		return host, ""
	}
	if strings.Count(host, ":") == 1 {
		h, port, _ := strings.Cut(host, ":")
		return h, port
	}
	return host, ""
}

// hostArgs are the arguments of ssh that name host.
func hostArgs(host string) []string {
	h, port := SSHTarget(host)
	if port != "" {
		return []string{"-p", port, h}
	}
	return []string{h}
}

// SSHTarget is host as ssh takes it, [user@]host with no brackets, and
// its port, "" for none.
func SSHTarget(host string) (target, port string) {
	h, port := splitPort(host)
	return strings.NewReplacer("[", "", "]", "").Replace(h), port
}

// quote quotes s for the POSIX shell.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
