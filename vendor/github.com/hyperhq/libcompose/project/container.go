package project

// Container defines what a libcompose container provides.
type Container interface {
	ID() (string, error)
	Name() string
	Port(port string) (string, error)
	IsRunning() (bool, error)
}
