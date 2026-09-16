package chat

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
)

// LocalProvider answers from the folder description alone, with no model and
// no network. It finds the files whose name or content holds the words of the
// question. A person with no API key still asks where something is.
type LocalProvider struct{}

// Name returns the provider name.
func (p *LocalProvider) Name() string { return ProviderLocal }

// Answer writes the files that match the last question.
func (p *LocalProvider) Answer(_ context.Context, req Request, w io.Writer) (string, error) {
	question := ""
	if len(req.Messages) > 0 {
		question = req.Messages[len(req.Messages)-1].Text
	}
	files := parseFileList(req.System)
	bodies := parseFileBodies(req.System)
	answer := searchFolder(question, files, bodies)
	if _, err := io.WriteString(w, answer); err != nil {
		return "", err
	}
	return answer, nil
}

type fileHit struct {
	path  string
	score int
	lines []string
}

// searchFolder builds the answer for one question.
func searchFolder(question string, files []string, bodies map[string]string) string {
	words := keywords(question)
	if len(words) == 0 {
		return listFiles(files)
	}

	hits := map[string]*fileHit{}
	for _, path := range files {
		hit := &fileHit{path: path}
		lower := strings.ToLower(path)
		for _, word := range words {
			if strings.Contains(lower, word) {
				hit.score += 3
			}
		}
		if hit.score > 0 {
			hits[path] = hit
		}
	}
	for path, body := range bodies {
		hit := hits[path]
		if hit == nil {
			hit = &fileHit{path: path}
		}
		for _, line := range strings.Split(body, "\n") {
			lower := strings.ToLower(line)
			matched := false
			for _, word := range words {
				if strings.Contains(lower, word) {
					hit.score++
					matched = true
				}
			}
			if matched && len(hit.lines) < 3 {
				hit.lines = append(hit.lines, strings.TrimSpace(line))
			}
		}
		if hit.score > 0 {
			hits[path] = hit
		}
	}

	ranked := make([]*fileHit, 0, len(hits))
	for _, hit := range hits {
		ranked = append(ranked, hit)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].path < ranked[j].path
	})

	if len(ranked) == 0 {
		return fmt.Sprintf("No file in this folder holds %s. The folder holds %d files.",
			strings.Join(words, " or "), len(files))
	}
	if len(ranked) > 5 {
		ranked = ranked[:5]
	}

	var b strings.Builder
	b.WriteString("These files match:\n")
	for _, hit := range ranked {
		fmt.Fprintf(&b, "- %s\n", hit.path)
		for _, line := range hit.lines {
			if line != "" {
				fmt.Fprintf(&b, "    %s\n", trim(line, 120))
			}
		}
	}
	b.WriteString("\nThis answer comes from a word search. Set an API key for a full answer.")
	return b.String()
}

func listFiles(files []string) string {
	if len(files) == 0 {
		return "This folder holds no files."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "This folder holds %d files:\n", len(files))
	shown := files
	if len(shown) > 20 {
		shown = shown[:20]
	}
	for _, f := range shown {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	if len(files) > len(shown) {
		fmt.Fprintf(&b, "- and %d more\n", len(files)-len(shown))
	}
	return strings.TrimRight(b.String(), "\n")
}

var stopWords = map[string]bool{
	"the": true, "and": true, "what": true, "where": true, "which": true,
	"who": true, "how": true, "does": true, "this": true, "that": true,
	"with": true, "for": true, "are": true, "was": true, "you": true,
	"folder": true, "file": true, "files": true, "there": true, "about": true,
	"tell": true, "show": true, "give": true, "list": true, "hold": true,
	"holds": true, "have": true, "has": true, "here": true, "any": true,
	"all": true, "can": true, "say": true, "says": true, "inside": true,
}

// keywords returns the words of a question that carry meaning.
func keywords(question string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range strings.FieldsFunc(strings.ToLower(question), func(r rune) bool {
		return !('a' <= r && r <= 'z') && !('0' <= r && r <= '9') && r != '.' && r != '-' && r != '_'
	}) {
		word := strings.Trim(raw, ".-_")
		if len(word) < 3 || stopWords[word] || seen[word] {
			continue
		}
		seen[word] = true
		out = append(out, word)
	}
	return out
}

// parseFileList reads the file names out of a folder description.
func parseFileList(system string) []string {
	var out []string
	inList := false
	for _, line := range strings.Split(system, "\n") {
		switch {
		case strings.HasPrefix(line, "Files:"):
			inList = true
			continue
		case inList && strings.HasPrefix(line, "- "):
			name := strings.TrimPrefix(line, "- ")
			if i := strings.LastIndex(name, " ("); i > 0 {
				name = name[:i]
			}
			if strings.HasPrefix(name, "and ") {
				continue
			}
			out = append(out, name)
		case inList && strings.TrimSpace(line) == "":
			continue
		case inList:
			inList = false
		}
	}
	return out
}

// parseFileBodies reads the file content out of a folder description.
func parseFileBodies(system string) map[string]string {
	out := map[string]string{}
	parts := strings.Split(system, "\n--- ")
	for _, part := range parts[1:] {
		end := strings.Index(part, " ---\n")
		if end < 0 {
			continue
		}
		path := part[:end]
		body := part[end+len(" ---\n"):]
		out[path] = body
	}
	return out
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
