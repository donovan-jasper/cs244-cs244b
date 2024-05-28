package tests

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"raftlog"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
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
		t.Errorf("Failed to write entry to WAL, file size is 0")
	}

	// print contents of file
	file, err := os.Open(filepath)
	if err != nil {
		t.Errorf("Failed to open file")
	}
	defer file.Close()
	var chunksize uint32
	newEntry := &raftlog.LogEntry{}
	binary.Read(file, binary.LittleEndian, &chunksize)
	data := make([]byte, chunksize)
	if _, err := io.ReadFull(file, data); err != nil {
		t.Errorf("Failed to read data")
	}
	// decode data
	if err := proto.Unmarshal(data, newEntry); err != nil {
		t.Error("failed to unmarshal entry", err)
	}

	// Check if the entry was written correctly to the WAL
	if newEntry.Term != entry.Term || newEntry.Index != entry.Index || newEntry.Command != entry.Command {
		t.Errorf("Failed to write entry correctly to WAL. Expected: %v, Got: %v", entry, newEntry)
	}
}

func TestLoadLog(t *testing.T) {
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

	// Load the log from the WAL
	raftLog.LoadLog()

	// Check if the log was loaded successfully
	if raftLog.GetSize() != 1 {
		t.Errorf("Failed to load log from WAL")
	}
}
