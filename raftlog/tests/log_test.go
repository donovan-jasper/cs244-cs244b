package tests

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"raftlog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func SameEntryDetails(a *raftlog.LogEntry, b *raftlog.LogEntry) bool {
	return a.Term == b.Term && a.Index == b.Index && a.Command == b.Command
}

func TestCreateRaftLog(t *testing.T) {
	// Create a RaftLog object
	ex, err := os.Executable()
	if err != nil {
		t.Errorf("Failed to get executable path")
	}
	filepath := filepath.Join(filepath.Dir(ex), "test_wal")
	raftLog := raftlog.NewRaftLog(filepath, false)

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
	rl := raftlog.NewRaftLog(filepath, false)

	// Append an entry to the RaftLog
	entry := &raftlog.LogEntry{
		Term:    1,
		Index:   1,
		Command: "test",
	}
	rl.AppendEntry(entry)

	// Check if the entry was appended successfully
	if rl.GetSize() != 1 {
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
	if !SameEntryDetails(entry, newEntry) {
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
	rl := raftlog.NewRaftLog(filepath, false)

	// Append an entry to the RaftLog
	entry := &raftlog.LogEntry{
		Term:    1,
		Index:   1,
		Command: "test",
	}
	rl.AppendEntry(entry)

	// Load the log from the WAL
	rl.LoadLog()

	// Check if the log was loaded successfully
	if rl.GetSize() != 1 {
		t.Errorf("Failed to load log from WAL")
	}
}

func TestDeleteEntries(t *testing.T) {
	// Create a RaftLog object
	ex, err := os.Executable()
	if err != nil {
		t.Errorf("Failed to get executable path")
	}
	filepath := filepath.Join(filepath.Dir(ex), "test_wal")
	rl := raftlog.NewRaftLog(filepath, false)

	for i := 1; i <= 5; i++ {
		// Append an entry to the RaftLog
		entry := &raftlog.LogEntry{
			Term:    1,
			Index:   int32(i),
			Command: fmt.Sprintf("test%v", i),
		}
		rl.AppendEntry(entry)
	}

	// Truncate the log
	rl.DeleteEntries(3)

	// Check if the log was truncated successfully
	assert.Equal(t, 2, rl.GetSize(), "Failed to truncate log")
}
