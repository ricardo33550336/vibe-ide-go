package main

import "os"

// commandLine reports whether Vibe was started as the vibe command,
// which on Linux is Vibe itself: with arguments, unless detach or a
// snapshot started it.
func commandLine() bool {
	return os.Getenv(cliEnv) != "" || len(os.Args) > 1 && os.Getenv(detachedEnv) == "" && os.Getenv("VIBE_SNAPSHOT") == ""
}

// attachConsole does nothing: Linux starts with the command's handles.
func attachConsole() {}

// startApp starts Vibe apart from the command, opening paths.
func startApp(paths []string) error { return startDetached(paths, detachedEnv+"=1") }

// refreshCLI does nothing: the command is Vibe itself.
func refreshCLI() {}
