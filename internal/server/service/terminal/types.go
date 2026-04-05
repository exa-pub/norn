package terminal

// TerminalSession is an ephemeral interactive shell session.
type TerminalSession struct {
	ID           string
	InstanceName string
	Name         string
	TTYID        string // reference to underlying TTY Session
}
