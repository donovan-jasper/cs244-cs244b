package tests

import (
	"os"
	"path/filepath"
	"raftlog"
	"testing"
	"time"
)

func TestCreateRaftLog(t *testing.T) {
	// Create a RaftLog object
	ex, err := os.Executable()
	if err != nil {
		t.Errorf("Failed to get executable path")
	}
	filepath := filepath.Join(filepath.Dir(ex), "test_wal")
	raftLog := raftlog.NewRaftLog(filepath)

	// Check if the RaftLog object is not nil
	if raftLog == nil {
		t.Errorf("Failed to create RaftLog object")
	}
}

func TestAppendEntry(t *testing.T) {
	// Create a RaftLog object
	ex, err := os.Executable()
	if err != nil {
		t.Errorf("Failed to get executable path")
	}
	filepath := filepath.Join(filepath.Dir(ex), "test_wal")
	raftLog := raftlog.NewRaftLog(filepath)

	// Append an entry to the RaftLog
	entry := &raftlog.LogEntry{
		Term:    1,
		Index:   1,
		Command: "test",
	}
	raftLog.AppendEntry(entry)

	// Check if the entry was appended successfully
	if raftLog.GetSize() != 1 {
		t.Errorf("Failed to append entry to RaftLog")
	}

	time.Sleep(1000 * time.Millisecond)
	// check if the entry was written to the WAL
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		t.Errorf("Failed to get file info")
	}
	if fileInfo.Size() == 0 {
		t.Errorf("Failed to write entry to WAL")
	}
}
