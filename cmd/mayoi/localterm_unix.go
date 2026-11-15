//go:build darwin || linux

package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/creack/pty"
)

// localPTY is a terminal's program on this machine, run by us rather than
// by MyGo's pty: on macOS, reading the master through os.File of MyGo's
// own pseudo-terminal returns EAGAIN at once, which ends the program with
// SIGHUP. creack/pty opens it the way the runtime polls, and a read that
// ends keeps the screen until the terminal closes, so that the app shows
// the exit rather than MyGo's "[Disconnected]".
type localPTY struct {
	f     *os.File
	cmd   *exec.Cmd
	once  sync.Once
	mu    sync.Mutex
	gone  chan struct{}
	onEnd func(int)
	ended bool
	code  int
}

// startLocalPTY starts the shell of a local terminal in dir.
func startLocalPTY(dir string) (*localPTY, error) {
	path, args, err := localShell()
	if err != nil {
		return nil, err
	}
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	// argv[0] is the login shell's name, with the dash: the program is
	// still path, which Go keeps in Cmd.Path.
	cmd := exec.Command(path)
	cmd.Args = args
	cmd.Dir = dir
	cmd.Env = terminalEnv()
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		return nil, err
	}
	p := &localPTY{f: f, cmd: cmd, gone: make(chan struct{})}
	go p.wait()
	return p, nil
}

// localShell is the shell a new terminal runs: $SHELL as a login shell, as
// terminal apps do, else zsh on macOS and sh elsewhere.
func localShell() (string, []string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
		if runtime.GOOS == "darwin" {
			shell = "/bin/zsh"
		}
	}
	path, err := exec.LookPath(shell)
	if err != nil {
		return "", nil, err
	}
	return path, []string{"-" + filepath.Base(path)}, nil
}

// terminalEnv is the environment of a terminal's program: this process's,
// with the variables terminals set.
func terminalEnv() []string {
	env := os.Environ()
	set := func(kv string) {
		k, _, _ := strings.Cut(kv, "=")
		for i, e := range env {
			if strings.HasPrefix(e, k+"=") {
				env[i] = kv
				return
			}
		}
		env = append(env, kv)
	}
	set("TERM=xterm-256color")
	set("COLORTERM=truecolor")
	if os.Getenv("LANG") == "" && os.Getenv("LC_ALL") == "" && os.Getenv("LC_CTYPE") == "" {
		set("LANG=en_US.UTF-8")
	}
	return env
}

func (p *localPTY) wait() {
	err := p.cmd.Wait()
	code := 0
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		code = exit.ExitCode()
	case err != nil:
		code = -1
	}
	p.mu.Lock()
	p.ended, p.code = true, code
	fn := p.onEnd
	p.mu.Unlock()
	if fn != nil {
		fn(code)
	}
}

// setOnEnd calls fn once the program ended, or at once when it did before.
func (p *localPTY) setOnEnd(fn func(int)) {
	p.mu.Lock()
	p.onEnd = fn
	ended, code := p.ended, p.code
	p.mu.Unlock()
	if ended {
		fn(code)
	}
}

func (p *localPTY) Read(b []byte) (int, error) {
	n, err := p.f.Read(b)
	if err != nil {
		// The program ended, or the terminal closed: keep what it showed
		// until the terminal closes, as a terminal app leaves the last
		// screen.
		<-p.gone
		return 0, io.EOF
	}
	return n, nil
}

func (p *localPTY) Write(b []byte) (int, error) { return p.f.Write(b) }

func (p *localPTY) Close() error {
	p.once.Do(func() { close(p.gone) })
	err := p.f.Close()
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	return err
}

func (p *localPTY) Resize(cols, rows int) error {
	return pty.Setsize(p.f, &pty.Winsize{Cols: uint16(max(cols, 1)), Rows: uint16(max(rows, 1))})
}

func (p *localPTY) ResizePixels(cols, rows, cellW, cellH int) error {
	return pty.Setsize(p.f, &pty.Winsize{
		Cols: uint16(max(cols, 1)), Rows: uint16(max(rows, 1)),
		X: uint16(max(cols*cellW, 0)), Y: uint16(max(rows*cellH, 0)),
	})
}
