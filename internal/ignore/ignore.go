// Package ignore decides which files a folder does not sync.
package ignore

import (
	"bufio"
	"os"
	"path"
	"strings"
)

// FileName is the optional file in the folder root that holds extra patterns.
const FileName = ".lynxignore"

// Defaults are always active. They hold the state Lynx keeps for itself, the
// files a repository manages itself, and the files an operating system writes
// without being asked.
var Defaults = []string{
	".lynx/",
	".git/",
	".hg/",
	".svn/",
	"node_modules/",
	".DS_Store",
	"Thumbs.db",
	"desktop.ini",
	"*.swp",
	"*~",
	".lynx-tmp-*",
}

// Rule is one pattern from a list.
type Rule struct {
	pattern  string
	segments []string
	negate   bool
	dirOnly  bool
	anchored bool
}

// List is an ordered set of rules. The last rule that matches decides.
type List struct {
	rules []Rule
}

// New builds a list from pattern lines.
func New(patterns []string) *List {
	l := &List{}
	for _, raw := range patterns {
		if r, ok := parse(raw); ok {
			l.rules = append(l.rules, r)
		}
	}
	return l
}

// Load builds a list from the defaults plus the folder's ignore file. A
// missing ignore file is not an error.
func Load(root string) (*List, error) {
	patterns := append([]string{}, Defaults...)
	f, err := os.Open(path.Join(root, FileName))
	if err != nil {
		if os.IsNotExist(err) {
			return New(patterns), nil
		}
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		patterns = append(patterns, s.Text())
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return New(patterns), nil
}

func parse(raw string) (Rule, bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return Rule{}, false
	}
	r := Rule{pattern: line}
	if strings.HasPrefix(line, "!") {
		r.negate = true
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		r.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if strings.HasPrefix(line, "/") {
		r.anchored = true
		line = strings.TrimPrefix(line, "/")
	}
	if line == "" {
		return Rule{}, false
	}
	if strings.Contains(line, "/") {
		r.anchored = true
	}
	r.segments = strings.Split(line, "/")
	return r, true
}

// Match reports whether a path inside the folder is ignored. The path is
// relative to the folder root and uses forward slashes.
func (l *List) Match(rel string, isDir bool) bool {
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return false
	}
	parts := strings.Split(rel, "/")

	ignored := false
	for _, r := range l.rules {
		if r.covers(parts, isDir) {
			ignored = !r.negate
		}
	}
	return ignored
}

// covers reports whether the rule matches the path itself or any parent
// directory of it. A rule that hides a directory hides its contents too.
func (r Rule) covers(parts []string, isDir bool) bool {
	for n := 1; n < len(parts); n++ {
		if r.matchAt(parts, n) {
			return true
		}
	}
	if r.dirOnly && !isDir {
		return false
	}
	return r.matchAt(parts, len(parts))
}

// matchAt matches the rule against the first n segments of the path.
func (r Rule) matchAt(parts []string, n int) bool {
	head := parts[:n]
	if r.anchored {
		return matchSegments(r.segments, head)
	}
	for start := 0; start < len(head); start++ {
		if matchSegments(r.segments, head[start:]) {
			return true
		}
	}
	return false
}

// matchSegments matches pattern segments against path segments. The segment
// "**" matches any number of segments.
func matchSegments(pattern, parts []string) bool {
	if len(pattern) == 0 {
		return len(parts) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(parts); i++ {
			if matchSegments(pattern[1:], parts[i:]) {
				return true
			}
		}
		return false
	}
	if len(parts) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], parts[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pattern[1:], parts[1:])
}
