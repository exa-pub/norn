package entity

// TTYSession is an ephemeral PTY session inside a container.
type TTYSession struct {
	ID   string // session ID for WebSocket routing
	Name string // optional human-readable name
}
