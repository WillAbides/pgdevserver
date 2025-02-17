package pgtestserver

type Status int

const (
	StatusUnknown Status = iota
	StatusStopped
	StatusRunning
	StatusInvalid
)

func (s Status) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
	case StatusStopped:
		return "stopped"
	case StatusRunning:
		return "running"
	default:
		return "invalid"
	}
}
