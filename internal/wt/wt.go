// Package wt reads Worktrunk worktree information by shelling out to the wt CLI.
package wt

import (
	"encoding/json"
	"fmt"
	"os/exec"
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
