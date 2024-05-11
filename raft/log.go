package log

// creating raft log entry
type logEntry struct {
	term    int
	index   int         // ensure ordering
	command interface{} // change to bytes?
}

func createLogEntry(term int, index int, command interface{}) *logEntry {
	return &logEntry{
		term:    term,
		index:   index,
		command: command,
	}
}
