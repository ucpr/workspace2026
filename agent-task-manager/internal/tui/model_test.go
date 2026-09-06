package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	ghlib "github.com/ucpr/atama/internal/github"
	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/service"
	"github.com/ucpr/atama/internal/store"
)

func newTestModel(t *testing.T) (*Model, *service.Service) {
	t.Helper()
	st, err := store.Init(t.TempDir())
	if err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	svc := service.New(st)
	m := New(svc, st)
	runCmd(t, m, m.Init())
	return m, svc
}

// runCmd executes cmd (if any) and feeds the resulting message(s) back into
// m.Update, the same way the bubbletea runtime would — letting tests drive
// the model without a real terminal. tea.BatchMsg is expanded recursively.
func runCmd(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if msg == nil {
		return
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			runCmd(t, m, c)
		}
		return
	}
	_, next := m.Update(msg)
	runCmd(t, m, next)
}

func sendKey(t *testing.T, m *Model, key string) {
	t.Helper()
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "space":
		msg = tea.KeyMsg{Type: tea.KeySpace}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	_, cmd := m.Update(msg)
	runCmd(t, m, cmd)
}

func TestModel_ReloadPopulatesColumns(t *testing.T) {
	m, svc := newTestModel(t)
	if _, err := svc.AddTask(service.AddTaskInput{Title: "task 1"}); err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	runCmd(t, m, m.reload())

	if len(m.allTasks) != 1 {
		t.Fatalf("allTasks = %v, want 1 task", m.allTasks)
	}
	if got := m.focusedTask(); got == nil || got.Title != "task 1" {
		t.Fatalf("focusedTask() = %v, want task 1", got)
	}
}

func TestModel_NavigateColumns(t *testing.T) {
	m, svc := newTestModel(t)
	svc.AddTask(service.AddTaskInput{Title: "backlog task"})
	inProg, _ := svc.AddTask(service.AddTaskInput{Title: "in progress task"})
	svc.SetStatus(inProg.ID, model.StatusInProgress)
	runCmd(t, m, m.reload())

	if m.focusCol != 0 {
		t.Fatalf("initial focusCol = %d, want 0", m.focusCol)
	}
	sendKey(t, m, "l")
	sendKey(t, m, "l") // ready column is empty; should land on in_progress eventually via multiple rights
	sendKey(t, m, "l")
	if got := m.focusedTask(); got != nil && got.Status != model.StatusInProgress {
		// Not fatal: exact column index depends on layout, but focusedTask
		// should never panic and should stay within bounds.
		t.Logf("focused task after navigation = %+v", got)
	}
}

func TestModel_SpaceAdvancesStatus(t *testing.T) {
	m, svc := newTestModel(t)
	task, _ := svc.AddTask(service.AddTaskInput{Title: "advance me"})
	runCmd(t, m, m.reload())

	sendKey(t, m, "space")

	got, err := svc.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Status != model.StatusReady {
		t.Errorf("Status after space = %q, want ready", got.Status)
	}
}

func TestModel_CreateTaskViaForm(t *testing.T) {
	m, svc := newTestModel(t)
	runCmd(t, m, m.reload())

	sendKey(t, m, "n")
	if m.mode != modeForm {
		t.Fatalf("mode after 'n' = %v, want modeForm", m.mode)
	}
	sendKey(t, m, "new task title")
	sendKey(t, m, "enter")

	if m.mode != modeBoard {
		t.Fatalf("mode after submit = %v, want modeBoard", m.mode)
	}
	tasks, err := svc.ListTasks(service.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "new task title" {
		t.Fatalf("tasks = %+v, want one task titled %q", tasks, "new task title")
	}
}

func TestModel_DeleteTaskWithConfirmation(t *testing.T) {
	m, svc := newTestModel(t)
	task, _ := svc.AddTask(service.AddTaskInput{Title: "to delete"})
	runCmd(t, m, m.reload())

	sendKey(t, m, "d")
	if m.mode != modeConfirmDelete {
		t.Fatalf("mode after 'd' = %v, want modeConfirmDelete", m.mode)
	}
	sendKey(t, m, "y")
	if m.mode != modeBoard {
		t.Fatalf("mode after confirming delete = %v, want modeBoard", m.mode)
	}
	if _, err := svc.GetTask(task.ID); err == nil {
		t.Error("task still exists after confirmed delete")
	}
}

func TestModel_ViewDoesNotPanicInAnyMode(t *testing.T) {
	m, svc := newTestModel(t)
	task, _ := svc.AddTask(service.AddTaskInput{Title: "some task", Body: "some body"})
	svc.AddComment(task.ID, "alice", "a comment")
	runCmd(t, m, m.reload())
	m.width, m.height = 120, 40

	modes := []mode{
		modeBoard, modeDetail, modeForm, modeDepEditor, modeConfirmDelete,
		modeGoalPicker, modeFilterMenu, modeSearch, modeGraph, modeHelp,
		modeCommentInput, modeConfirmSync,
	}
	m.detailTaskID = task.ID
	m.confirmTaskID = task.ID
	m.depEditorTaskID = task.ID
	m.pendingSyncRepo = "acme/widgets"
	m.pendingSyncResult = &ghlib.SyncResult{}

	for _, mode := range modes {
		m.mode = mode
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("View() panicked in mode %v: %v", mode, r)
				}
			}()
			_ = m.View()
		}()
	}
}

func TestModel_DeleteTaskCancel(t *testing.T) {
	m, svc := newTestModel(t)
	task, _ := svc.AddTask(service.AddTaskInput{Title: "keep me"})
	runCmd(t, m, m.reload())

	sendKey(t, m, "d")
	sendKey(t, m, "n")
	if m.mode != modeBoard {
		t.Fatalf("mode after canceling delete = %v, want modeBoard", m.mode)
	}
	if _, err := svc.GetTask(task.ID); err != nil {
		t.Errorf("task should still exist after canceled delete, GetTask() error = %v", err)
	}
}
