// Package worktreesidebar renders the worktree details and git diff for the
// worktrees view sidebar.
package worktreesidebar

import (
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/worktreessection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

type DiffFetchedMsg struct {
	Branch string
	Diff   string
	Err    error
}

type Model struct {
	ctx     *context.ProgramContext
	row     *worktreessection.RowData
	diff    string
	err     error
	loading bool
}

func NewModel(ctx *context.ProgramContext) Model {
	return Model{ctx: ctx}
}

// SetRow selects a worktree row. Returns a command to fetch its diff when the
// selection changed to a different worktree, nil otherwise.
func (m *Model) SetRow(row *worktreessection.RowData) tea.Cmd {
	if row == nil {
		m.row = nil
		m.diff = ""
		return nil
	}

	if m.row != nil && m.row.Data.Branch == row.Data.Branch && m.row.Data.Path == row.Data.Path {
		return nil
	}

	m.row = row
	m.diff = ""
	m.err = nil
	m.loading = true

	branch := row.Data.Branch
	base := row.BaseBranch
	path := row.Data.Path
	isMain := row.Data.IsMain
	return func() tea.Msg {
		diff, err := fetchDiff(path, base, isMain)
		return DiffFetchedMsg{Branch: branch, Diff: diff, Err: err}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case DiffFetchedMsg:
		if m.row != nil && m.row.Data.Branch == msg.Branch {
			m.loading = false
			m.diff = msg.Diff
			m.err = msg.Err
		}
	}
	return m, nil
}

func runGit(path string, args ...string) (string, error) {
	c := exec.Command("git", append([]string{"-C", path}, args...)...)
	out, err := c.Output()
	return string(out), err
}

func fetchDiff(path, base string, isMain bool) (string, error) {
	var s strings.Builder

	if !isMain {
		committed, err := runGit(path, "diff", "--no-color", base+"...HEAD")
		if err != nil {
			return "", fmt.Errorf("diff %s...HEAD: %w", base, err)
		}
		if strings.TrimSpace(committed) != "" {
			s.WriteString(lipgloss.NewStyle().Bold(true).Render(
				fmt.Sprintf("COMMITTED (vs %s)", base)))
			s.WriteString("\n")
			s.WriteString(strings.TrimRight(committed, "\n"))
			s.WriteString("\n\n")
		}
	}

	working, err := runGit(path, "diff", "--no-color", "HEAD")
	if err != nil {
		return "", fmt.Errorf("diff HEAD: %w", err)
	}
	if strings.TrimSpace(working) != "" {
		s.WriteString(lipgloss.NewStyle().Bold(true).Render("UNCOMMITTED"))
		s.WriteString("\n")
		s.WriteString(strings.TrimRight(working, "\n"))
		s.WriteString("\n")
	}

	if s.Len() == 0 {
		return "No changes", nil
	}
	return s.String(), nil
}

func (m Model) View() string {
	if m.row == nil {
		return "No worktree selected"
	}

	s := strings.Builder{}
	s.WriteString(lipgloss.NewStyle().Bold(true).Render(m.row.Data.Branch))
	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
		Render(m.row.Data.Path))
	s.WriteString("\n\n")

	if m.loading {
		s.WriteString("\nLoading diff...")
		return s.String()
	}

	if m.err != nil {
		s.WriteString(lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText).
			Render(fmt.Sprintf("Error: %v", m.err)))
		return s.String()
	}

	width := m.ctx.DynamicPreviewWidth
	body := lipgloss.NewStyle().MaxWidth(max(width, 0)).Render(m.diff)
	s.WriteString(body)

	return s.String()
}
