package chat

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedFolder(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "# Weekend trip\nWe drive to the coast on Friday.\n")
	write("notes/budget.md", "Budget for the trip\n- hotel: 240 euro\n- fuel: 60 euro\n")
	write("photo.bin", "\x00\x01\x02binary bytes")
	return root
}

func describeSeeded(t *testing.T) string {
	t.Helper()
	root := seedFolder(t)
	entries, err := FolderEntries(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("the folder listed %d files, want 3", len(entries))
	}
	return Describe(root, "trip", entries)
}

func TestDescribeHoldsNamesAndText(t *testing.T) {
	system := describeSeeded(t)
	for _, want := range []string{"README.md", "notes/budget.md", "photo.bin", "240 euro", "Weekend trip"} {
		if !strings.Contains(system, want) {
			t.Errorf("the folder description misses %q", want)
		}
	}
	if strings.Contains(system, "binary bytes") {
		t.Error("the folder description holds the content of a file that is not text")
	}
}

func TestLocalProviderNamesARealFile(t *testing.T) {
	system := describeSeeded(t)
	session := NewSession(&LocalProvider{}, system)

	var out strings.Builder
	answer, err := session.Ask(context.Background(), "how much is the hotel?", &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(answer, "notes/budget.md") {
		t.Fatalf("the answer does not name the file that holds the price:\n%s", answer)
	}
	if out.String() != answer {
		t.Fatal("the written answer differs from the returned answer")
	}
	if session.Turns() != 1 {
		t.Fatalf("the session holds %d turns, want 1", session.Turns())
	}
}

func TestLocalProviderListsTheFolder(t *testing.T) {
	session := NewSession(&LocalProvider{}, describeSeeded(t))
	answer, err := session.Ask(context.Background(), "what is in this folder?", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"README.md", "notes/budget.md"} {
		if !strings.Contains(answer, want) {
			t.Errorf("the answer misses %q:\n%s", want, answer)
		}
	}
}

func TestLocalProviderSaysWhenNothingMatches(t *testing.T) {
	session := NewSession(&LocalProvider{}, describeSeeded(t))
	answer, err := session.Ask(context.Background(), "where is the submarine?", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(answer, "No file") {
		t.Fatalf("the answer invents something:\n%s", answer)
	}
}

func TestProviderChoiceFollowsTheKey(t *testing.T) {
	t.Setenv("LYNX_CHAT_PROVIDER", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	cfg := LoadConfig()
	if cfg.Provider != ProviderLocal {
		t.Fatalf("with no key the provider is %q, want %q", cfg.Provider, ProviderLocal)
	}
	if cfg.Model != DefaultModel {
		t.Fatalf("the model is %q, want %q", cfg.Model, DefaultModel)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	cfg = LoadConfig()
	if cfg.Provider != ProviderAnthropic {
		t.Fatalf("with a key the provider is %q, want %q", cfg.Provider, ProviderAnthropic)
	}
	p, err := NewProvider(cfg)
	if err != nil || p.Name() != ProviderAnthropic {
		t.Fatalf("NewProvider gave %v, %v", p, err)
	}
}

func TestUnknownProviderFails(t *testing.T) {
	if _, err := NewProvider(Config{Provider: "gpt"}); err == nil {
		t.Fatal("an unknown provider was accepted")
	}
}

func TestSessionForgetsAFailedTurn(t *testing.T) {
	session := NewSession(failing{}, "system")
	if _, err := session.Ask(context.Background(), "question", io.Discard); err == nil {
		t.Fatal("the failing provider did not fail")
	}
	if session.Turns() != 0 {
		t.Fatalf("the session kept %d turns after a failure, want 0", session.Turns())
	}
}

type failing struct{}

func (failing) Name() string { return "failing" }
func (failing) Answer(context.Context, Request, io.Writer) (string, error) {
	return "", io.ErrUnexpectedEOF
}
