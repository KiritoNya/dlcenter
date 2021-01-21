package main

type State string

const (
	download  State = "downloading"
	pause     State = "paused"
	inQueue   State = "queued"
	complete  State = "completed"
	terminate State = "terminated"
	//TODO: ERROR STATE
)

func (s State) String() string {
	return string(s)
}

func checkIfStateExist(state string) bool {
	switch State(state) {
	case download:
		return true
	case pause:
		return true
	case inQueue:
		return true
	case complete:
		return true
	case terminate:
		return true
	//TODO: Error state
	default:
		return false
	}
}