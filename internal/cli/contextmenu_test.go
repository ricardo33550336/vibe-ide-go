package cli

import "testing"

// TestContextMenus checks the keys and commands of the "Open with Vibe"
// item, which must match those the installer writes.
func TestContextMenus(t *testing.T) {
	exe := `C:\Users\Jane\AppData\Local\Programs\Vibe\Vibe.exe`
	want := map[string]string{
		`Software\Classes\*\shell\dev.vibe.editor`:                    `"` + exe + `" "%1"`,
		`Software\Classes\Directory\shell\dev.vibe.editor`:            `"` + exe + `" "%1"`,
		`Software\Classes\Directory\Background\shell\dev.vibe.editor`: `"` + exe + `" "%V"`,
	}
	if len(contextMenus) != len(want) {
		t.Fatalf("%d menus, want %d", len(contextMenus), len(want))
	}
	for _, m := range contextMenus {
		if got, ok := want[m.key()]; !ok || m.command(exe) != got {
			t.Errorf("%s runs %s, want %s", m.key(), m.command(exe), got)
		}
	}
}
