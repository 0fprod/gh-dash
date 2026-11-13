package worktreessection

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/table"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
	"github.com/dlvhdr/gh-dash/v4/internal/wt"
)

type RowData struct {
	Ctx        *context.ProgramContext
	Data       wt.Worktree
	BaseBranch string
	Columns    []table.Column
}

func (r *RowData) GetRepoNameWithOwner() string {
	return ""
}

func (r *RowData) GetTitle() string {
	return r.Data.Branch
}

func (r *RowData) GetNumber() int {
	return 0
}

func (r *RowData) GetUrl() string {
	return ""
}

func (r *RowData) GetUpdatedAt() time.Time {
	if r.Data.Commit.Timestamp <= 0 {
		return time.Time{}
	}
	return time.Unix(r.Data.Commit.Timestamp, 0)
}

func (r RowData) getTextStyle() lipgloss.Style {
	return components.GetIssueTextStyle(r.Ctx)
}

func (r RowData) getBaseStyle(isSelected bool) lipgloss.Style {
	baseStyle := lipgloss.NewStyle()
	if isSelected {
		baseStyle = baseStyle.Background(r.Ctx.Theme.SelectedBackground)
	}
	return baseStyle
}

func (r RowData) isDirty() bool {
	w := r.Data.WorkingTree
	return w.Staged || w.Modified || w.Untracked || w.Renamed || w.Deleted
}

func (r RowData) renderStatus(isSelected bool) string {
	if !r.isDirty() {
		return r.getBaseStyle(isSelected).
			Foreground(r.Ctx.Theme.SuccessText).
			Render("✓")
	}
	return r.getBaseStyle(isSelected).
		Foreground(r.Ctx.Styles.Colors.OpenPR).
		Render(r.Data.Symbols)
}

func (r RowData) renderBranch(isSelected bool) string {
	style := r.getBaseStyle(isSelected)
	name := r.Data.Branch
	switch {
	case r.Data.IsCurrent:
		name = style.Foreground(r.Ctx.Theme.SuccessText).Render("● " + name)
	case r.Data.IsMain:
		name = style.Foreground(r.Ctx.Theme.FaintText).Render(name)
	default:
		name = style.Foreground(r.Ctx.Theme.PrimaryText).Render(name)
	}
	return name
}

func (r RowData) renderLines(isSelected bool) string {
	added := max(r.Data.WorkingTree.Diff.Added, 0)
	deleted := max(r.Data.WorkingTree.Diff.Deleted, 0)
	if added == 0 && deleted == 0 {
		return r.getBaseStyle(isSelected).Foreground(r.Ctx.Theme.FaintText).Render("-")
	}

	baseStyle := r.getBaseStyle(isSelected)
	additions := baseStyle.Foreground(r.Ctx.Theme.SuccessText).Render(
		fmt.Sprintf("+%d", added))
	deletions := baseStyle.Foreground(r.Ctx.Theme.ErrorText).Render(
		fmt.Sprintf("-%d", deleted))
	return lipgloss.JoinHorizontal(lipgloss.Top, additions, " ", deletions)
}

func (r RowData) renderBaseState(isSelected bool) string {
	style := r.getBaseStyle(isSelected)
	switch {
	case r.Data.Ahead > 0 && r.Data.Behind > 0:
		return style.Foreground(r.Ctx.Styles.Colors.OpenPR).
			Render(fmt.Sprintf("↑%d↓%d", r.Data.Ahead, r.Data.Behind))
	case r.Data.Ahead > 0:
		return style.Foreground(r.Ctx.Theme.SuccessText).
			Render(fmt.Sprintf("↑%d", r.Data.Ahead))
	case r.Data.Behind > 0:
		return style.Foreground(r.Ctx.Theme.ErrorText).
			Render(fmt.Sprintf("↓%d", r.Data.Behind))
	case r.Data.IsMain:
		return style.Foreground(r.Ctx.Theme.FaintText).Render("-")
	default:
		return style.Foreground(r.Ctx.Theme.SuccessText).Render("✓")
	}
}

func (r RowData) renderPath(isSelected bool) string {
	p := r.Data.Path
	if home, err := homeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		p = "~" + p[len(home):]
	}
	return r.getBaseStyle(isSelected).
		Foreground(r.Ctx.Theme.FaintText).
		Render(filepath.ToSlash(p))
}

func (r RowData) renderUpdatedAt(isSelected bool) string {
	ts := r.Data.Commit.Timestamp
	if r.Data.LastMod > ts {
		ts = r.Data.LastMod
	}
	if ts <= 0 {
		return r.getBaseStyle(isSelected).Foreground(r.Ctx.Theme.FaintText).Render("-")
	}
	return r.getTextStyle().Render(utils.TimeElapsed(time.Unix(ts, 0)))
}

func (r RowData) ToTableRow(isSelected bool) table.Row {
	return table.Row{
		r.renderStatus(isSelected),
		r.renderBranch(isSelected),
		r.renderBaseState(isSelected),
		r.renderUpdatedAt(isSelected),
		r.renderPath(isSelected),
	}
}

func GetSectionColumns(
	ctx *context.ProgramContext,
	cfg config.PrsSectionConfig,
) []table.Column {
	return []table.Column{
		{Title: "", Width: new(int)},
		{Title: "Branch", Grow: boolPtr(true)},
		{Title: "Base", Width: intPtr(8)},
		{Title: "Updated", Width: intPtr(10)},
		{Title: "Path", Width: intPtr(38)},
	}
}
