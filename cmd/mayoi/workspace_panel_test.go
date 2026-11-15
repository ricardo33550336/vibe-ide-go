package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"mayoi/internal/settings"
)

// A terminal of the workspace panel shows its workspace in the center, and
// opening a file brings the editor, until a terminal is chosen again.
func TestWorkspacePanelShowsTerminalsAndEditors(t *testing.T) {
	root, other := workspaceFixture(t), workspaceFixture(t)
	s := settings.Default()
	s.ProjectTabs = true
	m := &mayoi{s: s}
	f := &frame{m: m}
	var hs []*desktop
	for _, dir := range []string{root, other} {
		w := s.NewWindow()
		w.Folder = dir
		h := &desktop{m: m}
		h.a = newApp(&fakeHost{}, s, w)
		h.a.terminalFirst, h.a.centerTerm = true, true
		t.Cleanup(h.a.shutdown)
		f.add(h)
		hs = append(hs, h)
	}
	f.active = hs[0]
	m.wins = []*desktop{hs[0], hs[1]}
	tt := ui.NewTester(f.view, 1200, 760)
	h := &harness{t: t, a: hs[0].a, tt: tt, root: root}
	h.settle()

	s0 := hs[0].a.startTerminal(root)
	s0.name = "alpha"
	hs[0].a.panelTerm = s0
	s1 := hs[1].a.startTerminal(other)
	s1.name = "beta"
	hs[1].a.panelTerm = s1
	h.settle()

	if err := tt.Click("beta"); err != nil {
		t.Fatal(err)
	}
	h.a = hs[1].a
	h.settle()
	if f.active != hs[1] || !hs[1].a.centerTerm || hs[1].a.panelTerm != s1 {
		t.Fatalf("beta did not show: active %p, center %v, panel %p", f.active, hs[1].a.centerTerm, hs[1].a.panelTerm)
	}

	hs[1].a.openFile(h.path("main.go"), true)
	h.settle()
	if hs[1].a.centerTerm {
		t.Fatal("a file did not bring the editors")
	}
	if _, ok := tt.Find("main.go"); !ok {
		t.Fatalf("no editor tab: %v", tt.Texts())
	}

	if err := tt.Click("beta"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !hs[1].a.centerTerm || hs[1].a.panelTerm != s1 {
		t.Fatalf("beta did not come back: center %v, panel %p", hs[1].a.centerTerm, hs[1].a.panelTerm)
	}
}

// A session keeps its workspace's terminals, and starts them again in
// their folders, showing the one that was active.
func TestWorkspacePanelRestoresTerminals(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	root, other := workspaceFixture(t), workspaceFixture(t)
	s := settings.Default()
	w := s.NewWindow()
	w.Folder = root
	w.Terminals = []settings.Term{
		{Dir: root, Name: "saved-one"},
		{Dir: other, Name: "saved-two"},
	}
	w.ActiveTerminal = 1
	a := newApp(&fakeHost{folder: root}, s, w)
	a.terminalFirst, a.centerTerm = true, true
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 1200, 760)
	h := &harness{t: t, a: a, tt: tt, root: root}
	h.settle()

	terms := a.panelTerms()
	if len(terms) != 2 {
		t.Fatalf("%d terminals, want 2", len(terms))
	}
	if terms[0].name != "saved-one" || terms[1].name != "saved-two" {
		t.Fatalf("names %q, %q", terms[0].name, terms[1].name)
	}
	if a.panelTerm != terms[1] {
		t.Fatalf("active terminal %v, want the second", a.panelTerm)
	}
	if terms[0].dir != root || terms[1].dir != other {
		t.Fatalf("folders %q, %q", terms[0].dir, terms[1].dir)
	}
}
