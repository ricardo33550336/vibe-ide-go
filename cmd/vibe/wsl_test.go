package main

import "testing"

func TestWSLPath(t *testing.T) {
	tests := []struct{ in, want string }{
		// A folder of a distribution opens there, read by wsl.exe.
		{`\\wsl.localhost\Ubuntu\home\me\proj`, "/wsl:Ubuntu/home/me/proj"},
		{`\\wsl$\Debian\srv\app\main.go`, "/wsl:Debian/srv/app/main.go"},
		{`\\WSL$\Ubuntu\home\me`, "/wsl:Ubuntu/home/me"},
		// Folders of this machine stay as they are: writing one through a
		// distribution writes a file of Windows from Linux.
		{`C:\Users\me\proj`, `C:\Users\me\proj`},
		{`/Users/me/proj`, "/Users/me/proj"},
		{`\\server\share\proj`, `\\server\share\proj`},
		{"", ""},
		{"/wsl:Ubuntu/home/me", "/wsl:Ubuntu/home/me"},
	}
	for _, tt := range tests {
		if got := wslPath(tt.in); got != tt.want {
			t.Errorf("wslPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
