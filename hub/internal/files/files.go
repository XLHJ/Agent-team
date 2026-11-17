// Package files implements the source-preview backend: a path-sandboxed
// file tree and file reader over one workspace root directory. The panel
// front end (Monaco-based, MIT) consumes these via panel.files.* methods.
package files

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxReadBytes caps one file read; larger files are truncated.
const MaxReadBytes = 2 << 20 // 2 MiB

// PreviewMaxLines caps returned lines; larger files are truncated.
const PreviewMaxLines = 20000

// ignored directories/files are never listed (VCS internals, build output).
var ignored = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true,
	"dist": true, "build": true, "out": true, "target": true,
	"__pycache__": true, ".venv": true, "venv": true, ".idea": true,
	".DS_Store": true,
}

// Entry is one row of a directory listing.
type Entry struct {
	Name string `json:"name"`
	Dir  bool   `json:"is_dir"`
	Size int64  `json:"size"`
}

// Read is one file preview result.
type Read struct {
	Content   string `json:"content"`
	Language  string `json:"language"`
	Size      int64  `json:"size"`
	Lines     int    `json:"lines"`
	Truncated bool   `json:"truncated"`
}

// Service serves file listings and reads under one root.
type Service struct {
	root string
}

// New builds a Service. Empty root disables the feature.
func New(root string) (*Service, error) {
	if root == "" {
		return nil, nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("workspace %s is not a directory", abs)
	}
	return &Service{root: abs}, nil
}

// Enabled reports whether a workspace root is configured.
func (s *Service) Enabled() bool { return s != nil && s.root != "" }

// Root returns the sandboxed absolute workspace root.
func (s *Service) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

// resolve maps a client-relative path onto the sandboxed absolute path.
// Every returned path has been verified to stay inside the root, so the
// caller can trust it for direct os calls.
func (s *Service) resolve(rel string) (string, error) {
	if !s.Enabled() {
		return "", errors.New("no workspace")
	}
	rel = strings.TrimPrefix(strings.ReplaceAll(rel, "\\", "/"), "/")
	if rel == "" {
		rel = "."
	}
	abs, err := filepath.EvalSymlinks(filepath.Join(s.root, filepath.Clean(rel)))
	if err != nil {
		return "", err
	}
	if abs != s.root && !strings.HasPrefix(abs, s.root+string(filepath.Separator)) {
		return "", errors.New("path outside workspace")
	}
	return abs, nil
}

// List returns the entries of one directory (dirs first, then files).
func (s *Service) List(rel string) ([]Entry, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, errors.New("not a directory")
	}
	dirents, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, d := range dirents {
		if ignored[d.Name()] {
			continue
		}
		e := Entry{Name: d.Name(), Dir: d.IsDir()}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				e.Size = info.Size()
			}
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Read returns one text file for preview. Binary files are rejected.
func (s *Service) Read(rel string) (*Read, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, errors.New("not a file")
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, 0, MaxReadBytes)
	chunk := make([]byte, 64<<10)
	for len(buf) <= MaxReadBytes {
		n, err := f.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}
		if err != nil {
			break
		}
	}
	if bytes.IndexByte(buf[:min(len(buf), 8192)], 0) >= 0 {
		return nil, errors.New("binary file")
	}
	content := string(buf)
	truncated := int64(len(buf)) < st.Size()
	lines := strings.Count(content, "\n") + 1
	if lines > PreviewMaxLines {
		idx := 0
		for i := 0; i < PreviewMaxLines; i++ {
			j := strings.IndexByte(content[idx:], '\n')
			if j < 0 {
				break
			}
			idx += j + 1
		}
		content = content[:idx]
		lines = PreviewMaxLines
		truncated = true
	}
	return &Read{
		Content:   content,
		Language:  LanguageOf(rel),
		Size:      st.Size(),
		Lines:     lines,
		Truncated: truncated,
	}, nil
}

// languages maps extensions to Monaco language ids (subset).
var languages = map[string]string{
	".go": "go", ".ts": "typescript", ".tsx": "typescript", ".mts": "typescript",
	".js": "javascript", ".jsx": "javascript", ".mjs": "javascript", ".cjs": "javascript",
	".json": "json", ".md": "markdown", ".py": "python", ".rs": "rust",
	".yaml": "yaml", ".yml": "yaml", ".toml": "ini", ".ini": "ini", ".cfg": "ini",
	".html": "html", ".htm": "html", ".css": "css", ".scss": "scss", ".less": "less",
	".sh": "shell", ".bash": "shell", ".zsh": "shell", ".ps1": "powershell",
	".sql": "sql", ".java": "java", ".kt": "kotlin", ".swift": "swift",
	".c": "c", ".h": "c", ".cpp": "cpp", ".hpp": "cpp", ".cs": "csharp",
	".rb": "ruby", ".php": "php", ".xml": "xml", ".vue": "html",
	".lua": "lua", ".pl": "perl", ".r": "r", ".dart": "dart", ".bat": "bat",
	".dockerfile": "dockerfile", ".proto": "proto", ".graphql": "graphql",
}

// LanguageOf guesses the Monaco language id from a file name.
func LanguageOf(name string) string {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "dockerfile") {
		return "dockerfile"
	}
	if strings.HasPrefix(lower, "makefile") {
		return "shell"
	}
	ext := filepath.Ext(lower)
	if lang, ok := languages[ext]; ok {
		return lang
	}
	return "plaintext"
}
