package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap documents the vim-like bindings from requirements §5.5.1. It's
// only used to render the help overlay; actual dispatch happens in each
// mode's update function by matching msg.String().
type keyMap struct {
	Left, Right key.Binding
	Up, Down    key.Binding
	Enter       key.Binding
	Space       key.Binding
	JumpLeft    key.Binding
	JumpRight   key.Binding
	New         key.Binding
	Edit        key.Binding
	Delete      key.Binding
	DepEditor   key.Binding
	Graph       key.Binding
	GoalFilter  key.Binding
	FilterMenu  key.Binding
	Sort        key.Binding
	Search      key.Binding
	Sync        key.Binding
	Import      key.Binding
	Tab         key.Binding
	Comment     key.Binding
	Reload      key.Binding
	Help        key.Binding
	Esc         key.Binding
	Quit        key.Binding
}

var keys = keyMap{
	Left:       key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "focus left column")),
	Right:      key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "focus right column")),
	Up:         key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "focus up")),
	Down:       key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "focus down")),
	Enter:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open detail")),
	Space:      key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "advance status")),
	JumpLeft:   key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "move task left")),
	JumpRight:  key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "move task right")),
	New:        key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new task")),
	Edit:       key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit task")),
	Delete:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete task")),
	DepEditor:  key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "edit dependencies")),
	Graph:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "dependency graph")),
	GoalFilter: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "filter by goal")),
	FilterMenu: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter by label/priority")),
	Sort:       key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle sort")),
	Search:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Sync:       key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "github sync")),
	Import:     key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "github import")),
	Tab:        key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch detail tab")),
	Comment:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "add comment")),
	Reload:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload")),
	Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Esc:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close/clear")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

func helpLines() []string {
	return []string{
		"h/← l/→   focus column      j/↓ k/↑    focus task",
		"enter     open detail       space      advance status",
		"H / L     move task         n          new task",
		"e         edit task         d          delete task",
		"D         edit dependencies g          dependency graph",
		"G         filter by goal    f          filter label/priority",
		"s         cycle sort        /          search",
		"S         github sync       i          github import",
		"c         add comment       tab        switch detail tab",
		"r         reload            ?          toggle help",
		"esc       close/clear       q / ctrl+c quit",
	}
}
