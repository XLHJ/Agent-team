// Package detect scans PATH against the YAML signature table and produces
// intake candidates. No CLI name is hard-coded in Go; adapters only add or
// edit YAML entries (Phase-2 4.2).
package detect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Takeover methods in priority order.
const (
	MethodNative = "native" // CLI already speaks JSON-RPC; harness connects
	MethodStdio  = "stdio"  // structured stdio (NDJSON / stream-json / ACP)
	MethodPTY    = "pty"    // ConPTY / PTY fallback
)

// Signature is one table entry.
type Signature struct {
	Name         string   `yaml:"name"`
	DisplayName  string   `yaml:"display_name"`
	Executables  []string `yaml:"executables"`
	VersionArgs  []string `yaml:"version_args"`
	VersionRegex string   `yaml:"version_regex"`
	Methods      []string `yaml:"methods"`
	// Args are the structured-stdio launch arguments (Phase-3 adapters refine).
	Args []string `yaml:"args"`
	// Native transport hints: "stdio" (default) or "ws".
	Native string `yaml:"native"`
	// Approval mode the adapter intends to use.
	ApprovalMode string `yaml:"approval_mode"`
}

// Candidate is a detected CLI awaiting user confirmation.
type Candidate struct {
	Signature   string   `json:"signature"`
	DisplayName string   `json:"display_name"`
	Path        string   `json:"path"`
	Version     string   `json:"version"`
	Methods     []string `json:"methods"`
	Confidence  string   `json:"confidence"` // high | low
}

// Table is a loaded signature table.
type Table struct {
	Signatures []Signature `yaml:"signatures"`
}

// Load reads every *.yaml under dir.
func Load(dir string) (*Table, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("detect: no signature files in %s", dir)
	}
	t := &Table{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var part Table
		if err := yaml.Unmarshal(raw, &part); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		t.Signatures = append(t.Signatures, part.Signatures...)
	}
	sort.Slice(t.Signatures, func(i, j int) bool { return t.Signatures[i].Name < t.Signatures[j].Name })
	return t, nil
}

// Scan walks PATH for every signature and returns matches.
func Scan(t *Table) []Candidate {
	var out []Candidate
	seen := map[string]bool{}
	for _, sig := range t.Signatures {
		path, ok := findExecutable(sig.Executables)
		if !ok || seen[path] {
			continue
		}
		seen[path] = true
		c := Candidate{
			Signature:   sig.Name,
			DisplayName: sig.DisplayName,
			Path:        path,
			Methods:     sig.Methods,
			Confidence:  "low",
		}
		if v, ok := probeVersion(path, sig); ok {
			c.Version = v
			c.Confidence = "high"
		}
		out = append(out, c)
	}
	return out
}

// findExecutable resolves the first hit across PATH respecting PATHEXT.
func findExecutable(names []string) (string, bool) {
	exts := []string{""}
	if runtime.GOOS == "windows" {
		exts = []string{".exe", ".cmd", ".bat", ".ps1", ""}
	}
	for _, name := range names {
		for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
			for _, ext := range exts {
				p := filepath.Join(dir, name+ext)
				if isExecutable(p) {
					return p, true
				}
			}
		}
	}
	return "", false
}

func isExecutable(p string) bool {
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true // PATHEXT filtered above
	}
	return st.Mode()&0o111 != 0
}

func probeVersion(path string, sig Signature) (string, bool) {
	if len(sig.VersionArgs) == 0 || sig.VersionRegex == "" {
		return "", false
	}
	re, err := regexp.Compile(sig.VersionRegex)
	if err != nil {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := CommandSourceContext(ctx, path, sig.VersionArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return "", false
	}
	if m := re.FindStringSubmatch(string(out)); len(m) > 1 {
		return m[1], true
	}
	return "", false
}

// Executable returns a signature by name.
func (t *Table) Executable(name string) (Signature, error) {
	for _, s := range t.Signatures {
		if s.Name == name {
			return s, nil
		}
	}
	return Signature{}, errors.New("detect: unknown signature " + name)
}

// CommandSource builds an *exec.Cmd for path, unwrapping npm .cmd/.bat shims on
// Windows: resolve the shim's target script and run it with node when possible,
// otherwise fall back to `cmd /c shim args`.
func CommandSource(path string, args ...string) *exec.Cmd {
	if runtime.GOOS != "windows" {
		return exec.Command(path, args...)
	}
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".cmd") || strings.HasSuffix(lower, ".bat") {
		if script, ok := shimTarget(path); ok {
			return exec.Command("node", append([]string{script}, args...)...)
		}
		all := append([]string{"/c", path}, args...)
		return exec.Command("cmd", all...)
	}
	return exec.Command(path, args...)
}

// shimTarget reads an npm shim to find the JS entry it invokes.
func shimTarget(shim string) (string, bool) {
	raw, err := os.ReadFile(shim)
	if err != nil {
		return "", false
	}
	re := regexp.MustCompile(`"%~dp0\\([^"]+)"`)
	m := re.FindStringSubmatch(string(raw))
	if len(m) < 2 {
		return "", false
	}
	p := filepath.Join(filepath.Dir(shim), filepath.FromSlash(m[1]))
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}

// CommandSourceContext is CommandSource bound to ctx.
func CommandSourceContext(ctx context.Context, path string, args ...string) *exec.Cmd {
	base := CommandSource(path, args...)
	cmd := exec.CommandContext(ctx, base.Path, base.Args[1:]...)
	cmd.SysProcAttr = base.SysProcAttr
	return cmd
}
