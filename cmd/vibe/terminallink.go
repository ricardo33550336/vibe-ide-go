package main

import (
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"vibe/internal/remote"
	"vibe/internal/vfs"
)

// terminalFile opens a printed path without consulting the search index,
// so files Git ignores can be opened too. Relative paths are the shell's,
// falling back to the folder the terminal started in when it reports none.
// A terminal of a distribution of WSL prints paths of that Linux, which
// are the editor's through the folder of the terminal.
func (a *app) terminalFile(s *session, link string) bool {
	if strings.Contains(link, "://") || strings.HasPrefix(link, "mailto:") {
		return false
	}
	conn := sessionConn(s)
	reported := ""
	if s.term != nil {
		reported = s.term.Dir()
	}
	resolve := func(name string) string {
		if conn == nil {
			dir := s.dir
			if filepath.IsAbs(reported) {
				dir = reported
			}
			if strings.HasPrefix(name, "~/") || strings.HasPrefix(name, `~\`) {
				name = filepath.Join(homeDir(), name[2:])
			}
			if !filepath.IsAbs(name) {
				name = filepath.Join(dir, name)
			}
			return filepath.Clean(name)
		}
		// The shell's own paths: of its home, and of the folder it said it
		// was in, or of the one the terminal started in.
		_, dir, _ := remote.Split(s.dir)
		if strings.HasPrefix(reported, "/") {
			dir = reported
		}
		if strings.HasPrefix(name, "~/") {
			home := conn.Info().Home
			if home == "" {
				home = dir
			}
			name = path.Join(home, name[2:])
		}
		if !strings.HasPrefix(name, "/") {
			name = path.Join(dir, name)
		}
		return conn.Path(name)
	}
	found := func(name string) (os.FileInfo, error) { return vfs.Stat(name) }
	file := resolve(link)
	info, err := found(file)
	line, col := 0, 0
	if err != nil {
		// Prefer a literal filename if it exists, even one ending in :12.
		name, l, c, ok := cutLineSuffix(link)
		if !ok {
			return false
		}
		line, col = l, c
		file = resolve(name)
		info, err = found(file)
	}
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	a.openFile(file, true)
	if d := a.activeEditor(); line > 0 && d != nil && d.path == file {
		a.goToLine(line, col)
	}
	return true
}

// cutLineSuffix peels :line or :line:column, both 1-based, off the end of
// name. A column of 0, which some tools print, goes to the line.
func cutLineSuffix(name string) (rest string, line, col int, ok bool) {
	at := strings.LastIndexByte(name, ':')
	if at < 0 {
		return name, 0, 0, false
	}
	line, err := strconv.Atoi(name[at+1:])
	if err != nil || line < 0 {
		return name, 0, 0, false
	}
	rest = name[:at]
	if at = strings.LastIndexByte(rest, ':'); at >= 0 {
		if n, err := strconv.Atoi(rest[at+1:]); err == nil && n > 0 {
			line, col, rest = n, line, rest[:at]
		}
	}
	return rest, line, col, line >= 1
}
