// Package worktreesidebar renders the worktree details, changed files and git
// diff for the worktrees section sidebar.
package worktreesidebar

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/carousel"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/worktreessection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

var worktreeTabs = []string{" Files", " Diff"}

type DiffFetchedMsg struct {
	Branch string
	Diff   string
	Err    error
}

type FilesFetchedMsg struct {
	Branch string
	Files  []FileEntry
	Err    error
}

// FileDiffFetchedMsg carries the diff preview of a single changed file.
type FileDiffFetchedMsg struct {
	Branch string
	Path   string
	Diff   string
	Err    error
}

// FileEntry is one changed file of a worktree.
type FileEntry struct {
	Path    string
	Status  string // A, M, D, R, ...
	Added   int
	Deleted int
}

type Model struct {
	ctx      *context.ProgramContext
	row      *worktreessection.RowData
	carousel carousel.Model

	diff    string
	err     error
	loading bool

	files        []FileEntry
	filesErr     error
	filesLoading bool

	fileCursor     int
	fileDiffs      map[string]string
	fileDiffErr    map[string]error
	fileDiffByPath string // path whose diff is being fetched
	fileDiffSetFor map[string]bool
	expanded       map[string]bool
}

func NewModel(ctx *context.ProgramContext) Model {
	c := carousel.NewModel(
		carousel.WithItems(worktreeTabs),
		carousel.WithWidth(ctx.DynamicPreviewWidth),
	)
	return Model{
		ctx:           ctx,
		carousel:      c,
		fileDiffs:     map[string]string{},
		fileDiffErr:   map[string]error{},
		fileDiffSetFor: map[string]bool{},
		expanded:       map[string]bool{}, 
	}
}

// SetRow selects a worktree row. Returns a command to fetch its diff and
// changed files when the selection changed to a different worktree, nil
// otherwise.
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
	m.files = nil
	m.filesErr = nil
	m.filesLoading = true
	m.fileCursor = 0
	m.fileDiffs = map[string]string{}
	m.fileDiffErr = map[string]error{}
	m.fileDiffSetFor = map[string]bool{}
	m.expanded = map[string]bool{}
	m.fileDiffByPath = ""

	branch := row.Data.Branch
	base := row.BaseBranch
	path := row.Data.Path
	isMain := row.Data.IsMain

	diffCmd := func() tea.Msg {
		diff, err := fetchDiff(path, base, isMain)
		return DiffFetchedMsg{Branch: branch, Diff: diff, Err: err}
	}
	filesCmd := func() tea.Msg {
		files, err := fetchFiles(path, base, isMain)
		return FilesFetchedMsg{Branch: branch, Files: files, Err: err}
	}
	return tea.Batch(diffCmd, filesCmd)
}

// MoveTabLeft switches to the previous sidebar tab.
func (m *Model) MoveTabLeft() {
	m.carousel.MoveLeft()
}

// MoveTabRight switches to the next sidebar tab.
func (m *Model) MoveTabRight() {
	m.carousel.MoveRight()
}

// HandleKey processes file navigation keys while the Files tab is selected:
// J/K move the file cursor, Enter toggles the inline preview of the selected
// file (fetching its diff asynchronously the first time). Arrow up/down and
// j/k remain reserved for moving between worktree rows. Returns a command
// fetching the preview diff, or nil.
func (m *Model) HandleKey(msg tea.KeyMsg) tea.Cmd {
	if m.row == nil || m.carousel.SelectedItem() != worktreeTabs[0] {
		return nil
	}

	switch msg.String() {
	case "J":
		if m.fileCursor < len(m.files)-1 {
			m.fileCursor++
		}
		return nil
	case "K":
		if m.fileCursor > 0 {
			m.fileCursor--
		}
		return nil
	case "enter":
		if len(m.files) == 0 {
			return nil
		}
		file := m.files[m.fileCursor]
		if m.expanded[file.Path] {
			delete(m.expanded, file.Path)
			return nil
		}
		m.expanded[file.Path] = true
		return m.fetchFileDiffCmdFor(file.Path)
	default:
		return nil
	}
}

func (m *Model) fetchFileDiffCmdFor(filePath string) tea.Cmd {
	if m.row == nil {
		return nil
	}
	if _, ok := m.fileDiffSetFor[filePath]; ok {
		return nil
	}

	branch := m.row.Data.Branch
	base := m.row.BaseBranch
	path := m.row.Data.Path
	isMain := m.row.Data.IsMain
	m.fileDiffSetFor[filePath] = true
	m.fileDiffByPath = filePath

	return func() tea.Msg {
		diff, err := fetchFileDiff(path, base, isMain, filePath)
		return FileDiffFetchedMsg{Branch: branch, Path: filePath, Diff: diff, Err: err}
	}
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case DiffFetchedMsg:
		if m.row != nil && m.row.Data.Branch == msg.Branch {
			m.loading = false
			m.diff = msg.Diff
			m.err = msg.Err
		}

	case FilesFetchedMsg:
		if m.row != nil && m.row.Data.Branch == msg.Branch {
			m.filesLoading = false
			m.files = msg.Files
			m.filesErr = msg.Err
			m.fileCursor = 0
		}

	case FileDiffFetchedMsg:
		if m.row != nil && m.row.Data.Branch == msg.Branch {
			if msg.Err != nil {
				m.fileDiffErr[msg.Path] = msg.Err
			} else {
				m.fileDiffs[msg.Path] = msg.Diff
			}
			if m.fileDiffByPath == msg.Path {
				m.fileDiffByPath = ""
			}
		}
	}

	return nil
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

func fetchFileDiff(path, base string, isMain bool, filePath string) (string, error) {
	var s strings.Builder

	if !isMain {
		committed, err := runGit(path, "diff", "--no-color", base+"...HEAD", "--", filePath)
		if err != nil {
			return "", fmt.Errorf("diff %s...HEAD -- %s: %w", base, filePath, err)
		}
		if strings.TrimSpace(committed) != "" {
			s.WriteString(strings.TrimRight(committed, "\n"))
			s.WriteString("\n")
		}
	}

	working, err := runGit(path, "diff", "--no-color", "HEAD", "--", filePath)
	if err != nil {
		return "", fmt.Errorf("diff HEAD -- %s: %w", filePath, err)
	}
	if strings.TrimSpace(working) != "" {
		s.WriteString(strings.TrimRight(working, "\n"))
		s.WriteString("\n")
	}

	if s.Len() == 0 {
		return "No changes", nil
	}
	return s.String(), nil
}

// fetchFiles lists changed files with per-file line counts, merging committed
// and uncommitted changes.
func fetchFiles(path, base string, isMain bool) ([]FileEntry, error) {
	type rangeDiffs struct {
		numstat    string
		nameStatus string
	}

	ranges := []rangeDiffs{}
	if !isMain {
		numstat, err := runGit(path, "diff", "--no-color", "--numstat", base+"...HEAD")
		if err != nil {
			return nil, fmt.Errorf("numstat %s...HEAD: %w", base, err)
		}
		nameStatus, err := runGit(path, "diff", "--no-color", "--name-status", base+"...HEAD")
		if err != nil {
			return nil, fmt.Errorf("name-status %s...HEAD: %w", base, err)
		}
		ranges = append(ranges, rangeDiffs{numstat: numstat, nameStatus: nameStatus})
	}

	numstat, err := runGit(path, "diff", "--no-color", "--numstat", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("numstat HEAD: %w", err)
	}
	nameStatus, err := runGit(path, "diff", "--no-color", "--name-status", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("name-status HEAD: %w", err)
	}
	ranges = append(ranges, rangeDiffs{numstat: numstat, nameStatus: nameStatus})

	byPath := map[string]*FileEntry{}
	var order []string

	statusFor := make(map[string]string)
	for _, r := range ranges {
		for _, line := range strings.Split(strings.TrimRight(r.nameStatus, "\n"), "\n") {
			if line == "" {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) < 2 {
				continue
			}
			status := strings.TrimRight(parts[0], "0123456789")
			target := parts[len(parts)-1]
			statusFor[target] = status
			if _, ok := byPath[target]; !ok {
				byPath[target] = &FileEntry{Path: target, Status: status}
				order = append(order, target)
			}
		}

		for _, line := range strings.Split(strings.TrimRight(r.numstat, "\n"), "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) < 3 {
				continue
			}
			target := parts[2]
			entry, ok := byPath[target]
			if !ok {
				entry = &FileEntry{Path: target, Status: statusFor[target]}
				byPath[target] = entry
				order = append(order, target)
			}
			entry.Added += parseNum(parts[0])
			entry.Deleted += parseNum(parts[1])
		}
	}

	files := make([]FileEntry, 0, len(order))
	for _, p := range order {
		files = append(files, *byPath[p])
	}
	return files, nil
}

func parseNum(s string) int {
	if s == "-" {
		return 0
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// highlight renders a git diff with syntax highlighting.
func (m *Model) highlight(diff string, width int) string {
	if diff == "" {
		return ""
	}

	lexer := lexers.Get("diff")
	style := styles.Get("github")
	if m.ctx.HasDarkBackground {
		style = styles.Get("github-dark")
	}
	formatter := formatters.Get("terminal256")
	if lexer == nil || style == nil || formatter == nil {
		return diff
	}

	iterator, err := lexer.Tokenise(nil, diff)
	if err != nil {
		return diff
	}

	buf := &bytes.Buffer{}
	if err := formatter.Format(buf, style, iterator); err != nil {
		return diff
	}

	return lipgloss.NewStyle().MaxWidth(max(width, 0)).Render(buf.String())
}

func (m *Model) renderFileRow(file FileEntry, selected bool, width int) string {
	additions := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.SuccessText).
		Width(5).
		Render(fmt.Sprintf("+%d", file.Added))
	deletions := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.ErrorText).
		Width(5).
		Render(fmt.Sprintf("-%d", file.Deleted))

	statusColor := m.ctx.Theme.FaintText
	switch file.Status {
	case "A":
		statusColor = m.ctx.Theme.SuccessText
	case "D":
		statusColor = m.ctx.Theme.ErrorText
	case "M", "R", "C":
		statusColor = m.ctx.Theme.SecondaryText
	}
	status := lipgloss.NewStyle().
		Foreground(statusColor).
		Width(2).
		Render(file.Status)

	cursor := "  "
	if selected {
		cursor = "▸ "
	}

	prefix := lipgloss.JoinHorizontal(lipgloss.Top, cursor, status, additions, deletions, " ")
	remaining := width - lipgloss.Width(prefix)
	path := file.Path
	if len(path) > remaining && remaining > 0 {
		path = path[:max(remaining-1, 0)] + "…"
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, prefix, path)
	if selected {
		row = lipgloss.NewStyle().Bold(true).Render(row)
	}
	return row
}

func (m *Model) View() string {
	if m.row == nil {
		return "No worktree selected"
	}

	width := m.ctx.DynamicPreviewWidth
	m.carousel.SetWidth(width)

	s := strings.Builder{}
	s.WriteString(lipgloss.NewStyle().Bold(true).Render(m.row.Data.Branch))
	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
		Render(m.row.Data.Path))
	s.WriteString("\n\n")
	s.WriteString(m.carousel.View())
	s.WriteString("\n")

	switch m.carousel.SelectedItem() {
	case worktreeTabs[0]: // Files
		s.WriteString(m.viewFilesTab(width))
	default: // Diff
		s.WriteString(m.viewDiffTab(width))
	}

	return s.String()
}

func (m *Model) viewFilesTab(width int) string {
	s := strings.Builder{}

	if m.filesErr != nil {
		s.WriteString(lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText).
			Render(fmt.Sprintf("Error: %v", m.filesErr)))
		return s.String()
	}

	if m.filesLoading {
		s.WriteString("\nLoading files...")
		return s.String()
	}

	if len(m.files) == 0 {
		s.WriteString("\nNo changes")
		return s.String()
	}

	for i, file := range m.files {
		s.WriteString(m.renderFileRow(file, i == m.fileCursor, width))
		s.WriteString("\n")
		if m.expanded[file.Path] {
			s.WriteString(m.renderFilePreview(file, width))
			s.WriteString("\n")
		}
	}

	summary := fmt.Sprintf("%d files · J/K file · Enter preview · ctrl+d/u scroll",
		len(m.files))
	s.WriteString(lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
		Render(summary))

	return s.String()
}

func (m *Model) renderFilePreview(file FileEntry, width int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(m.ctx.Theme.FaintBorder).
		PaddingLeft(2)

	var body string
	if err, ok := m.fileDiffErr[file.Path]; ok {
		body = lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText).
			Render(fmt.Sprintf("Error: %v", err))
	} else if diff, ok := m.fileDiffs[file.Path]; ok {
		body = m.highlight(diff, width)
	} else {
		body = lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).
			Render("Loading preview...")
	}

	return style.Render(body)
}

func (m *Model) viewDiffTab(width int) string {
	s := strings.Builder{}

	if m.loading {
		s.WriteString("\nLoading diff...")
		return s.String()
	}

	if m.err != nil {
		s.WriteString(lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText).
			Render(fmt.Sprintf("Error: %v", m.err)))
		return s.String()
	}

	s.WriteString(m.highlight(m.diff, width))
	return s.String()
}

var _ = key.NewBinding
