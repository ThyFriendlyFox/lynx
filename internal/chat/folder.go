package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/thyfriendlyfox/lynx/internal/ignore"
	"github.com/thyfriendlyfox/lynx/internal/index"
)

// Budgets for the folder description that goes to the model.
const (
	// MaxTotalBytes is how much file content the description holds.
	MaxTotalBytes = 200 * 1024
	// MaxFileBytes is how much of one file the description holds.
	MaxFileBytes = 32 * 1024
	// MaxListed is how many file names the description lists.
	MaxListed = 500
)

// Describe writes the text that tells the model what is in the folder. It
// lists every file and then holds the content of the text files, newest
// first, until the budget is full.
func Describe(root, name string, entries []index.Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You answer questions about one folder on a person's computer.\n")
	fmt.Fprintf(&b, "The folder is named %q. It holds %d files.\n\n", name, len(entries))
	b.WriteString("Rules:\n")
	b.WriteString("- Answer from the folder content below. Name the files you used.\n")
	b.WriteString("- Say that you do not know when the content below does not answer the question.\n")
	b.WriteString("- Do not invent a file name. Only the names in the list exist.\n")
	b.WriteString("- Keep the answer short.\n\n")

	listed := entries
	if len(listed) > MaxListed {
		listed = listed[:MaxListed]
	}
	b.WriteString("Files:\n")
	for _, e := range listed {
		fmt.Fprintf(&b, "- %s (%s)\n", e.Path, humanSize(e.Size))
	}
	if len(entries) > len(listed) {
		fmt.Fprintf(&b, "- and %d more files\n", len(entries)-len(listed))
	}

	byNewest := make([]index.Entry, len(entries))
	copy(byNewest, entries)
	sort.Slice(byNewest, func(i, j int) bool { return byNewest[i].ModTime > byNewest[j].ModTime })

	b.WriteString("\nFile content:\n")
	used := 0
	for _, e := range byNewest {
		if used >= MaxTotalBytes {
			break
		}
		if e.Size == 0 || e.Size > MaxFileBytes {
			continue
		}
		body, ok := readText(filepath.Join(root, filepath.FromSlash(e.Path)))
		if !ok {
			continue
		}
		if used+len(body) > MaxTotalBytes {
			body = body[:MaxTotalBytes-used]
		}
		fmt.Fprintf(&b, "\n--- %s ---\n%s\n", e.Path, body)
		used += len(body)
	}
	if used == 0 {
		b.WriteString("\nNo file in this folder holds readable text.\n")
	}
	return b.String()
}

// readText returns the content of a file when the file holds text.
func readText(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	if len(data) == 0 || !utf8.Valid(data) {
		return "", false
	}
	// A file with a zero byte is a program or a picture, not text.
	for _, c := range data[:min(len(data), 8000)] {
		if c == 0 {
			return "", false
		}
	}
	return string(data), true
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FolderEntries lists the files of a folder for the chat. It reads names,
// sizes and times only, so a large folder costs no hashing.
func FolderEntries(root string) ([]index.Entry, error) {
	ign, err := ignore.Load(root)
	if err != nil {
		return nil, err
	}
	stats, err := index.Scan(root, ign)
	if err != nil {
		return nil, err
	}
	entries := make([]index.Entry, 0, len(stats))
	for _, st := range stats {
		entries = append(entries, index.Entry{
			Path:    st.Path,
			Size:    st.Size,
			ModTime: st.ModTime,
			Mode:    st.Mode,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}
