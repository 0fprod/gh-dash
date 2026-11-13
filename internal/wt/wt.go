// Package wt reads Worktrunk worktree information by shelling out to the wt CLI.
package wt

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Commit describes the current commit checked out in a worktree.
type Commit struct {
	Sha       string `json:"sha"`
	ShortSha  string `json:"short_sha"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// Diff counts the working tree line changes reported by wt list.
type Diff struct {
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
}

// WorkingTree describes the dirtiness of a worktree.
type WorkingTree struct {
	Staged    bool `json:"staged"`
	Modified  bool `json:"modified"`
	Untracked bool `json:"untracked"`
	Renamed   bool `json:"renamed"`
	Deleted   bool `json:"deleted"`
	Diff      Diff `json:"diff"`
}

// Worktree is one entry of `wt list --format json` (schema 1).
type Worktree struct {
	Branch      string      `json:"branch"`
	Path        string      `json:"path"`
	Kind        string      `json:"kind"`
	Commit      Commit      `json:"commit"`
	WorkingTree WorkingTree `json:"working_tree"`
	MainState   string      `json:"main_state"`
	IsMain      bool        `json:"is_main"`
	IsCurrent   bool        `json:"is_current"`
	Symbols     string      `json:"symbols"`

	// Enriched fields, computed locally from git (not part of wt output).
	Ahead   int   `json:"-"` // commits on this branch missing from the base branch
	Behind  int   `json:"-"` // commits on the base branch missing from this branch
	LastMod int64 `json:"-"` // unix ts of newest modified/untracked file, 0 if clean
}

// FetchWorktrees returns the worktrees known to Worktrunk for the repo at
// repoPath. An empty repoPath lets wt detect the repository from the current
// working directory.
func FetchWorktrees(repoPath string) ([]Worktree, error) {
	args := []string{}
	if repoPath != "" {
		args = append(args, "-C", repoPath)
	}
	args = append(args, "list", "--format", "json")

	out, err := exec.Command("wt", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("wt list: %w", err)
	}

	var worktrees []Worktree
	if err := json.Unmarshal(out, &worktrees); err != nil {
		return nil, fmt.Errorf("parsing wt list output: %w", err)
	}
	return worktrees, nil
}

// gitOut runs git in repoPath and returns trimmed stdout; errors are
// non-fatal (missing branches, permission issues, ...) and return 0.
func gitOut(repoPath string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", repoPath}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// EnrichWorktrees fills Ahead/Behind relative to the base branch (the branch
// of the is-main worktree) and LastMod (newest mtime among dirty and
// untracked files) for each worktree. Best effort: enrichment errors are
// ignored and fields stay at zero values.
func EnrichWorktrees(repoPath string, worktrees []Worktree) {
	if repoPath == "" {
		return
	}
	base := ""
	for i := range worktrees {
		if worktrees[i].IsMain {
			base = worktrees[i].Branch
		}
	}
	if base == "" {
		return
	}
	for i := range worktrees {
		w := &worktrees[i]
		if w.IsMain || w.Branch == base {
			continue
		}
		if n := gitOut(repoPath, "rev-list", "--count", base+".."+w.Branch); n != "" {
			fmt.Sscanf(n, "%d", &w.Ahead)
		}
		if n := gitOut(repoPath, "rev-list", "--count", w.Branch+".."+base); n != "" {
			fmt.Sscanf(n, "%d", &w.Behind)
		}
		enrichLastMod(w)
	}
}

// enrichLastMod stats every dirty/untracked file of the worktree and keeps
// the newest mtime.
func enrichLastMod(w *Worktree) {
	out := gitOut(w.Path, "status", "--porcelain", "-uall")
	if out == "" {
		return
	}
	var newest int64
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		p := line[3:]
		// Rename lines look like "R  old -> new".
		if i := strings.LastIndex(p, " -> "); i >= 0 {
			p = p[i+4:]
		}
		p = strings.Trim(p, `"`)
		info, err := os.Stat(filepath.Join(w.Path, filepath.FromSlash(p)))
		if err != nil {
			continue
		}
		if m := info.ModTime().Unix(); m > newest {
			newest = m
		}
	}
	if newest > w.LastMod {
		w.LastMod = newest
	}
}
