package main

import (
	"vibe/internal/remote"
)

// wslPath is a path of this machine as the editor knows it when it is a
// folder of a distribution of WSL: a path under \\wsl.localhost\<distro>,
// which Explorer and the command line hand over, is that distribution's
// own path, read there by wsl.exe rather than through the share Windows
// mounts it on, which is slow and goes through another system.
//
// A path elsewhere, as one of C:, is kept as it is: a distribution reads
// it at /mnt/c, but writing there changes a file of this machine through
// another system, which is not how the editor works. So is a path of this
// machine where WSL is not there at all.
func wslPath(p string) string {
	distro, rp, ok := remote.WSLFolderOfWindows(p)
	if !ok || distro == "" {
		return p
	}
	return remote.Path(remote.Key(remote.WSL, distro), rp)
}

// wslPaths is wslPath of each of paths.
func wslPaths(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = wslPath(p)
	}
	return out
}

// revealPath is how the file manager shows a path: one of this machine as
// it is, and one of a distribution of WSL through the share Windows mounts
// it on, which Explorer opens. A path of a host of ssh has none: "".
func revealPath(p string) string {
	if unc, ok := remote.WSLWindowsPath(p); ok {
		return unc
	}
	if remote.Is(p) {
		return ""
	}
	return p
}

// canReveal reports whether the file manager can show p.
func canReveal(p string) bool { return p != "" && revealPath(p) != "" }
