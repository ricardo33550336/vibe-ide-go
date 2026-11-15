package vfs

import (
	"encoding/json"
	"os"
	"testing"
)

func TestEnvironmentHelper(t *testing.T) {
	if os.Getenv("VIBE_VFS_ENV_HELPER") != "1" {
		return
	}
	cwd, _ := os.Getwd()
	_, unset := os.LookupEnv("VIBE_VFS_REMOVE")
	json.NewEncoder(os.Stdout).Encode(map[string]any{"cwd": cwd, "literal": os.Getenv("VIBE_VFS_LITERAL"), "override": os.Getenv("VIBE_VFS_OVERRIDE"), "removed": !unset})
	os.Exit(0)
}

func TestCommandEnvironment(t *testing.T) {
	t.Setenv("VIBE_VFS_REMOVE", "inherited")
	t.Setenv("VIBE_VFS_OVERRIDE", "old")
	root := t.TempDir()
	literal := "spaces, quotes \" and $` stay literal"
	cmd, err := CommandEnvironment(root, []string{"VIBE_VFS_ENV_HELPER=1", "VIBE_VFS_OVERRIDE=new", "VIBE_VFS_LITERAL=" + literal}, []string{"VIBE_VFS_REMOVE"}, nil, os.Args[0], "-test.run=^TestEnvironmentHelper$")
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Cwd, Literal, Override string
		Removed                bool
	}
	if err = json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	wantDir, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	actualDir, err := os.Stat(got.Cwd)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(wantDir, actualDir) || got.Literal != literal || got.Override != "new" || !got.Removed {
		t.Fatalf("child environment: %+v", got)
	}
	if os.Getenv("VIBE_VFS_REMOVE") != "inherited" || os.Getenv("VIBE_VFS_OVERRIDE") != "old" {
		t.Fatal("changed the parent environment")
	}
}
