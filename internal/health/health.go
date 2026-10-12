package health

// Level is how much a finding matters.
type Level int

const (
	// OK is a check that passed.
	OK Level = iota
	// Warning works but is not what setup would write.
	Warning
	// Problem stops Flashheart from working.
	Problem
)

// Finding is one result of a check, its text starting with what it checked.
type Finding struct {
	Level Level
	Text  string
}
