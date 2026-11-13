package worktreessection

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/search"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/table"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/wt"
)

const SectionType = "worktrees"

type Model struct {
	section.BaseModel
	Worktrees  []wt.Worktree
	BaseBranch string
}

func NewModel(
	id int,
	ctx *context.ProgramContext,
	cfg config.PrsSectionConfig,
	lastUpdated time.Time,
) Model {
	m := Model{}
	m.BaseModel = section.NewModel(
		ctx,
		section.NewSectionOptions{
			Id:          id,
			Config:      cfg.ToSectionConfig(),
			Type:        SectionType,
			Columns:     GetSectionColumns(ctx, cfg),
			Singular:    "worktree",
			Plural:      "worktrees",
			LastUpdated: lastUpdated,
		},
	)
	m.SearchBar = search.NewModel(ctx, search.SearchOptions{Placeholder: "Search worktrees..."})
	m.SearchValue = ""
	m.Worktrees = []wt.Worktree{}
	m.BaseBranch = "main"

	return m
}

// NewSection returns the Worktrees section, embedded as the last tab of
// the PRs view.
func NewSection(ctx *context.ProgramContext, id int) section.Section {
	cfg := config.PrsSectionConfig{Title: "Worktrees"}
	m := NewModel(id, ctx, cfg, time.Now())
	return &m
}

// FetchWorktreesMsg triggers a re-fetch of the worktrees list.
type FetchWorktreesMsg struct{}

type worktreesFetchedMsg struct {
	Worktrees []wt.Worktree
	Err       error
}

func (m *Model) fetchWorktreesCmd() tea.Cmd {
	return func() tea.Msg {
		wts, err := wt.FetchWorktrees(m.Ctx.RepoPath)
		return worktreesFetchedMsg{Worktrees: wts, Err: err}
	}
}



func (m *Model) Update(msg tea.Msg) (section.Section, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)

	switch msg := msg.(type) {
	case tea.KeyMsg:

		if m.IsSearchFocused() {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.SearchBar.SetValue(m.SearchValue)
				blinkCmd := m.SetIsSearching(false)
				return m, blinkCmd

			case "enter":
				m.Table.ResetCurrItem()
				m.SetIsSearching(false)
				m.SearchValue = m.SearchBar.Value()
				m.BuildRows()
				return m, nil
			}

			break
		}

	case FetchWorktreesMsg:
		loadingCmd := m.SetIsLoading(true)
		if loadingCmd != nil {
			cmds = append(cmds, loadingCmd)
		}
		cmds = append(cmds, m.fetchWorktreesCmd())

	case worktreesFetchedMsg:
		if msg.Err == nil {
			m.Worktrees = msg.Worktrees
			for _, w := range msg.Worktrees {
				if w.IsMain {
					m.BaseBranch = w.Branch
				}
			}
		}
		m.LastFetch = section.FetchEvent{At: time.Now()}
		stopLoading := m.SetIsLoading(false)
		if stopLoading != nil {
			cmds = append(cmds, stopLoading)
		}
	}

	search, searchCmd := m.SearchBar.Update(msg)
	cmds = append(cmds, searchCmd)
	m.SearchBar = search

	m.Table.SetRows(m.BuildRows())
	table, tableCmd := m.Table.Update(msg)
	m.Table = table
	cmds = append(cmds, tableCmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	if m.Table.Rows == nil {
		d := m.GetDimensions()
		return lipgloss.Place(
			d.Width,
			d.Height,
			lipgloss.Center,
			lipgloss.Center,
			"No worktrees found",
		)
	}

	return m.BaseModel.View()
}

func (m *Model) NumRows() int {
	return len(m.Worktrees)
}

func (m *Model) GetCurrRowData() *RowData {
	idx := m.Table.GetCurrItem()
	if idx < 0 || idx >= len(m.Worktrees) {
		return nil
	}
	return &RowData{
		Ctx:        m.Ctx,
		Data:       m.Worktrees[idx],
		BaseBranch: m.BaseBranch,
		Columns:    m.Table.Columns,
	}
}

func (m *Model) GetCurrRow() data.RowData {
	row := m.GetCurrRowData()
	if row == nil {
		return nil
	}
	return row
}

func (m *Model) FetchNextPageSectionRows() []tea.Cmd {
	if m == nil {
		return nil
	}
	return []tea.Cmd{m.fetchWorktreesCmd()}
}

func (m *Model) ResetRows() {
	m.Worktrees = nil
}

func (m *Model) GetItemSingularForm() string {
	return "Worktree"
}

func (m *Model) GetItemPluralForm() string {
	return "Worktrees"
}

func (m *Model) GetTotalCount() int {
	return len(m.Worktrees)
}

func (m *Model) GetPagerContent() string {
	return ""
}

func (m *Model) matchesFilter(w wt.Worktree) bool {
	f := strings.ToLower(strings.TrimSpace(m.SearchValue))
	if f == "" {
		return true
	}
	return strings.Contains(strings.ToLower(w.Branch), f) ||
		strings.Contains(strings.ToLower(w.Path), f) ||
		strings.Contains(strings.ToLower(w.Commit.Message), f)
}

func (m Model) BuildRows() []table.Row {
	rows := []table.Row{}
	currItem := m.Table.GetCurrItem()

	filtered := make([]wt.Worktree, 0)
	for _, w := range m.Worktrees {
		if m.matchesFilter(w) {
			filtered = append(filtered, w)
		}
	}

	for i, w := range filtered {
		row := RowData{Ctx: m.Ctx, Data: w, BaseBranch: m.BaseBranch, Columns: m.Table.Columns}
		rows = append(rows, row.ToTableRow(currItem == i))
	}

	return rows
}
