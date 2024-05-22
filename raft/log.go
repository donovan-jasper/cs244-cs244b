package raftlog

//accept and convert protobuf into LogEntry struct
// convert LogEntry into protobuf

// CALL 3 ENTRIES:
// append a LogEntry Protobuf (which will be a struct with protobuf functions)
// read a LogEntry (give an index, return)
// delete a LogEntry (give an index, return)

// creating raft log entry
type LogEntry struct {
	term    int    // term when entry was received by leader
	index   int    // ensure ordering
	command []byte // change to bytes?
}

type RaftLog struct {
	entries []LogEntry
}

// might return on failure
func (r *RaftLog) AppendEntry(entry LogEntry) {
	r.entries = append(r.entries, entry)
}

func (r *RaftLog) GetEntry(index int) LogEntry {
	return r.entries[index]
}

func (r *RaftLog) DeleteEntry(index int) {
	r.entries = append(r.entries[:index], r.entries[index+1:]...)
}
