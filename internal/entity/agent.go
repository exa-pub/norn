package entity

// AgentSession is a persistent Claude Code conversation session.
type AgentSession struct {
	ID      string // session UUID (for claude --resume)
	Name    string // human-readable
	Running bool   // is TTY currently active
}
