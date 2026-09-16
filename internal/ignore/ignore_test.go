package ignore

import "testing"

func TestDefaultsHideState(t *testing.T) {
	l := New(Defaults)
	cases := []struct {
		rel  string
		dir  bool
		want bool
	}{
		{".lynx", true, true},
		{".lynx/index.json", false, true},
		{".git/config", false, true},
		{"src/.git/config", false, true},
		{".DS_Store", false, true},
		{"notes/.DS_Store", false, true},
		{"node_modules/left-pad/index.js", false, true},
		{"notes.md", false, false},
		{"src/main.go", false, false},
		{"lynx-notes/plan.md", false, false},
	}
	for _, c := range cases {
		if got := l.Match(c.rel, c.dir); got != c.want {
			t.Errorf("Match(%q, dir=%v) = %v, want %v", c.rel, c.dir, got, c.want)
		}
	}
}

func TestPatterns(t *testing.T) {
	l := New([]string{"*.log", "/build/", "docs/**/draft.md", "secret/", "!keep.log"})
	cases := []struct {
		rel  string
		dir  bool
		want bool
	}{
		{"server.log", false, true},
		{"logs/server.log", false, true},
		{"keep.log", false, false},
		{"build/app", false, true},
		{"src/build/app", false, false},
		{"docs/a/b/draft.md", false, true},
		{"docs/draft.md", false, true},
		{"secret/keys.txt", false, true},
		{"README.md", false, false},
	}
	for _, c := range cases {
		if got := l.Match(c.rel, c.dir); got != c.want {
			t.Errorf("Match(%q) = %v, want %v", c.rel, got, c.want)
		}
	}
}
