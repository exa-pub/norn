package entity

// TerminalSession is an ephemeral interactive shell session.
// It wraps a TTY Session with instance context and a human-readable name.
type TerminalSession struct {
	ID           string // terminal session ID
	InstanceName string
	Name         string // optional human-readable
	TTYID        string // reference to underlying TTY Session
}
