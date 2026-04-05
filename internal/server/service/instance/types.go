package instance

import "time"

// ContainerStatus represents the observable state of an instance.
type ContainerStatus string

const (
	StatusStarting ContainerStatus = "starting"
	StatusRunning  ContainerStatus = "running"
	StatusStopped  ContainerStatus = "stopped"
	StatusError    ContainerStatus = "error"
)

// Instance is the complete observable view of a named environment.
type Instance struct {
	ID           string
	Name         string
	CreatedAt    time.Time
	DockerID     string          // from Docker API, empty if not found
	Status       ContainerStatus // computed from signals
	ErrorMessage string          // from last_error file
}
