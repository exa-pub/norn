package entity

import "time"

type ContainerStatus string

const (
	ContainerStatusStarting ContainerStatus = "starting"
	ContainerStatusRunning  ContainerStatus = "running"
	ContainerStatusStopped  ContainerStatus = "stopped"
	ContainerStatusError    ContainerStatus = "error"
)

type Container struct {
	ID           string // UUID
	Name         string // human-readable name
	DockerID     string
	Status       ContainerStatus
	ErrorMessage string
	CreatedAt    time.Time
}
