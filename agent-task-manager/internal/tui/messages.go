package tui

// statusMsg sets the transient status-bar line (e.g. a sync summary).
type statusMsg string

// errMsg surfaces a background operation's error in the status bar.
type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }
