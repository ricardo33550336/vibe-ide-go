//go:build !darwin && !linux && !windows

package main

import "errors"

// commandLine reports whether Vibe was started as the vibe command,
// which there is none of here.
func commandLine() bool { return false }

func attachConsole() {}

func startApp([]string) error { return errors.New("the vibe command is not supported here") }

func refreshCLI() {}
