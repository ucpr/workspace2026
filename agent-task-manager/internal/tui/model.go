// Package tui implements atama's kanban board (requirements §5.5), sharing
// the store/service used by the CLI so changes made in either are visible
// to the other (requirements §5.5: "CLIとデータストアを共有").
package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	ghlib "github.com/ucpr/atama/internal/github"
	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/service"
	"github.com/ucpr/atama/internal/store"
)

type mode int

const (
	modeBoard mode = iota
	modeDetail
	modeForm
	modeDepEditor
	modeConfirmDelete
	modeGoalPicker
	modeFilterMenu
	modeSearch
	modeGraph
	modeHelp
	modeCommentInput
	modeConfirmSync
)

// Model is the bubbletea root model for the board.
type Model struct {
	svc *service.Service
	st  *store.Store

	width, height int
	mode          mode

	allTasks  []*model.Task
	tasksByID map[string]*model.Task
	goals     []*model.Goal

	filters  filters
	sortMode sortMode

	columns  []column
	focusCol int
	focusRow int

	detailTaskID string
	detailTab    int // 0 body, 1 comments, 2 dependencies

	formTitle     textinput.Model
	formBody      textarea.Model
	formLabels    textinput.Model
	formAssignees textinput.Model
	formPriority  model.Priority
	formEditingID string // "" means creating a new task
	formFocusIdx  int

	depEditorTaskID     string
	depEditorCursor     int
	depEditorCandidates []*model.Task
	depEditorReturn     mode

	confirmTaskID string

	goalPickerCursor int

	filterLabelInput  textinput.Model
	filterMenuField   int
	filterPriorityIdx int

	searchInput textinput.Model

	commentInput textarea.Model

	pendingSyncResult *ghlib.SyncResult
	pendingSyncRepo   string

	statusBar string
	err       error
}

// priorityChoices is priorityFilter's cycle order; index 0 means "no
// priority filter".
var priorityChoices = []model.Priority{"", model.PriorityUrgent, model.PriorityHigh, model.PriorityMedium, model.PriorityLow}

// New builds the initial Model. st is used for GitHub operations, which
// need lower-level store access than service.Service exposes.
func New(svc *service.Service, st *store.Store) *Model {
	m := &Model{svc: svc, st: st}
	m.formTitle = textinput.New()
	m.formTitle.Placeholder = "title"
	m.formLabels = textinput.New()
	m.formLabels.Placeholder = "labels (comma separated)"
	m.formAssignees = textinput.New()
	m.formAssignees.Placeholder = "assignees (comma separated)"
	m.formBody = textarea.New()
	m.formBody.Placeholder = "body (Markdown)"
	m.filterLabelInput = textinput.New()
	m.filterLabelInput.Placeholder = "label"
	m.searchInput = textinput.New()
	m.searchInput.Placeholder = "search title/body"
	m.commentInput = textarea.New()
	m.commentInput.Placeholder = "comment body"
	return m
}

func (m *Model) Init() tea.Cmd {
	return m.reload()
}

// reloadedMsg carries fresh data loaded from the store.
type reloadedMsg struct {
	tasks []*model.Task
	goals []*model.Goal
}

func (m *Model) reload() tea.Cmd {
	return func() tea.Msg {
		tasks, err := m.svc.ListTasks(service.TaskFilter{})
		if err != nil {
			return errMsg{err}
		}
		goals, err := m.svc.ListGoals()
		if err != nil {
			return errMsg{err}
		}
		return reloadedMsg{tasks: tasks, goals: goals}
	}
}

func (m *Model) applyReload(msg reloadedMsg) {
	m.allTasks = msg.tasks
	m.goals = msg.goals
	m.tasksByID = make(map[string]*model.Task, len(msg.tasks))
	for _, t := range msg.tasks {
		m.tasksByID[t.ID] = t
	}
	m.rebuildColumns()
}

func (m *Model) rebuildColumns() {
	visible := applyFilters(m.allTasks, m.filters)
	m.columns = buildColumns(visible, m.tasksByID, m.sortMode)
	if m.focusCol >= len(m.columns) {
		m.focusCol = len(m.columns) - 1
	}
	if m.focusCol < 0 {
		m.focusCol = 0
	}
	m.clampFocusRow()
}

func (m *Model) clampFocusRow() {
	if m.focusCol < 0 || m.focusCol >= len(m.columns) {
		m.focusRow = 0
		return
	}
	n := len(m.columns[m.focusCol].Tasks)
	if m.focusRow >= n {
		m.focusRow = n - 1
	}
	if m.focusRow < 0 {
		m.focusRow = 0
	}
}

// focusedTask returns the task under the board cursor, or nil if the
// focused column is empty.
func (m *Model) focusedTask() *model.Task {
	if m.focusCol < 0 || m.focusCol >= len(m.columns) {
		return nil
	}
	col := m.columns[m.focusCol]
	if m.focusRow < 0 || m.focusRow >= len(col.Tasks) {
		return nil
	}
	return col.Tasks[m.focusRow]
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case reloadedMsg:
		m.applyReload(msg)
		return m, nil
	case formSubmittedMsg:
		m.applyReload(msg.reloadedMsg)
		m.mode = modeBoard
		m.blurForm()
		return m, nil
	case depsToggledMsg:
		m.applyReload(msg.reloadedMsg)
		return m, nil
	case commentSubmittedMsg:
		m.applyReload(msg.reloadedMsg)
		m.mode = modeDetail
		m.commentInput.Blur()
		return m, nil
	case deletedMsg:
		m.applyReload(msg.reloadedMsg)
		m.mode = modeBoard
		return m, nil
	case syncPreviewMsg:
		m.pendingSyncResult = msg.result
		m.pendingSyncRepo = msg.repo
		m.mode = modeConfirmSync
		return m, nil
	case syncAppliedMsg:
		m.applyReload(msg.reloadedMsg)
		m.statusBar = msg.summary
		m.err = nil
		m.mode = modeBoard
		return m, nil
	case errMsg:
		m.err = msg.err
		return m, nil
	case statusMsg:
		m.statusBar = string(msg)
		m.err = nil
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch m.mode {
	case modeDetail:
		return m.updateDetail(msg)
	case modeForm:
		return m.updateForm(msg)
	case modeDepEditor:
		return m.updateDepEditor(msg)
	case modeConfirmDelete:
		return m.updateConfirmDelete(msg)
	case modeGoalPicker:
		return m.updateGoalPicker(msg)
	case modeFilterMenu:
		return m.updateFilterMenu(msg)
	case modeSearch:
		return m.updateSearch(msg)
	case modeGraph:
		return m.updateGraph(msg)
	case modeHelp:
		return m.updateHelp(msg)
	case modeCommentInput:
		return m.updateCommentInput(msg)
	case modeConfirmSync:
		return m.updateConfirmSync(msg)
	default:
		return m.updateBoard(msg)
	}
}

func (m *Model) View() string {
	switch m.mode {
	case modeDetail:
		return m.viewDetail()
	case modeForm:
		return m.viewForm()
	case modeDepEditor:
		return m.viewDepEditor()
	case modeConfirmDelete:
		return m.viewConfirmDelete()
	case modeGoalPicker:
		return m.viewGoalPicker()
	case modeFilterMenu:
		return m.viewFilterMenu()
	case modeSearch:
		return m.viewSearch()
	case modeGraph:
		return m.viewGraph()
	case modeHelp:
		return m.viewHelp()
	case modeCommentInput:
		return m.viewCommentInput()
	case modeConfirmSync:
		return m.viewConfirmSync()
	default:
		return m.viewBoard()
	}
}

// Run starts the TUI, blocking until the user quits.
func Run(svc *service.Service, st *store.Store) error {
	p := tea.NewProgram(New(svc, st), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m *Model) ghAPI(ctx context.Context) (ghlib.API, string, error) {
	cfg, err := m.st.LoadConfig()
	if err != nil {
		return nil, "", err
	}
	if cfg.GitHub == nil || cfg.GitHub.Repo == "" {
		return nil, "", errNoGitHubRepo
	}
	token, err := ghlib.ResolveToken(ctx)
	if err != nil {
		return nil, "", err
	}
	client, err := ghlib.NewClient(token, cfg.GitHub.Repo)
	if err != nil {
		return nil, "", err
	}
	return client, cfg.GitHub.Repo, nil
}
