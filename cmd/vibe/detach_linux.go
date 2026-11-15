package main

import (
	"os"

	"github.com/egoist/mygo"
	"golang.org/x/sys/unix"
)

// detachedEnv marks the Vibe detach started, which the programs it runs,
// such as its terminals' shells, do not inherit.
const detachedEnv = "VIBE_DETACHED"

// detach starts Vibe again in a session of its own, away from the
// terminal it was started from, and reports whether it did: the shell has
// its prompt back at once, and closing the terminal leaves the windows, as
// the vibe command does with open on macOS. Development builds, and
// VIBE_FOREGROUND=1, stay in the terminal, which shows what they log.
func detach() bool {
	if os.Getenv(detachedEnv) != "" {
		os.Unsetenv(detachedEnv)
		return false
	}
	if os.Getenv("VIBE_FOREGROUND") != "" || !mygo.App.IsPackaged() || !onTerminal() {
		return false
	}
	return startDetached(os.Args[1:], detachedEnv+"=1") == nil
}

// onTerminal reports whether Vibe was started with a terminal for its
// input or output.
func onTerminal() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout, os.Stderr} {
		if _, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS); err == nil {
			return true
		}
	}
	return false
}
