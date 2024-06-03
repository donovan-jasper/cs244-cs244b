package raftlog

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"log"
	"os"
	"sync"
	"time"

	pb "raftprotos"

	"google.golang.org/protobuf/proto"
)

//accept and convert protobuf int32 LogEntry struct
// convert LogEntry int32 to protobuf

// CALL 3 ENTRIES:
// append a LogEntry Protobuf (which will be a struct with protobuf functions)
// read a LogEntry (give an index, return)
// delete a LogEntry (give an index, return)

// creating raft log entry
// type LogEntry struct {
// 	term    int32    // term when entry was received by leader
// 	index   int32    // ensure ordering
// 	command string // change to bytes?
// }

// wal of https://github.com/JyotinderSingh/go-wal as reference
// assume file is int32 (chunk size), then the data itself

var ()

type WAL struct {
	filename  string
	file      *os.File
	mu        sync.RWMutex
	bufWriter *bufio.Writer
	syncTimer *time.Timer
	ctx       context.Context
	metadata  []int // stores the indicies of the start of each entry
	curIndex  int   // current byte index in the file
}
type RaftLog struct {
	entries []*pb.LogEntry
	wal     *WAL
}

/**
 * 4 scenarios:
 * - file exists, loadBackup = false. Overwrite existing file
 * - file exists, loadBackup = true. Load from backup
 * - file does not exist, loadBackup = false. Create new file
 * - file does not exist, loadBackup = true. Throw error
 */
func NewWAL(filename string, loadBackup bool) *WAL {
	// only for writing
	createNew := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	loadWithBackup := os.O_RDWR | os.O_APPEND
	fileFlags := createNew

	if loadBackup {
		// check if file exists
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			log.Fatal("File does not exist")
		}
		fileFlags = loadWithBackup
	}

	file, err := os.OpenFile(filename, fileFlags, 0644)
	if err != nil {
		log.Fatal(err)
	}
	wal := &WAL{
		filename:  filename,
		file:      file,
		bufWriter: bufio.NewWriter(file),
		syncTimer: time.NewTimer(100 * time.Millisecond),
		ctx:       context.Background(),
		metadata:  make([]int, 0),
		curIndex:  0,
	}
	go wal.syncRoutine()
	return wal
}

func NewRaftLog(filename string, loadBackup bool) *RaftLog {
	wal := NewWAL(filename, loadBackup)
	raftLog := &RaftLog{
		entries: make([]*pb.LogEntry, 0),
		wal:     wal,
	}
	if loadBackup {
		entries, err := raftLog.wal.readAllEntries(raftLog.wal.file)
		if err != nil {
			log.Fatal(err)
		}
		raftLog.entries = entries
	}
	return raftLog
}

func (r *RaftLog) AppendEntry(entry *pb.LogEntry) {
	r.entries = append(r.entries, entry)
	r.wal.WriteEntry(entry)
}

func (r *RaftLog) GetEntry(index int32) *pb.LogEntry {
	return r.entries[index]
}

func (r *RaftLog) GetEntries() []*pb.LogEntry {
	return r.entries
}

func (r *RaftLog) GetLastEntry() *pb.LogEntry {
	return r.entries[len(r.entries)-1]
}

func (r *RaftLog) GetLastIndex() int32 {
	return int32(len(r.entries) - 1)
}

func (r *RaftLog) GetSize() int32 {
	return int32(len(r.entries))
}

func (r *RaftLog) DeleteEntries(index int32) {
	r.entries = r.entries[:index]
	// TODO: delete entry from WAL
	r.wal.TruncateAt(index)
}

func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.bufWriter.Flush()
}

func (w *WAL) syncRoutine() {
	for {
		select {
		case <-w.syncTimer.C:
			w.mu.Lock()
			err := w.Sync()
			w.mu.Unlock()
			if err != nil {
				log.Print("Error while syncing:", err)
			}
		case <-w.ctx.Done():
			return
		}
	}
}

func (w *WAL) WriteEntry(entry *pb.LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	// marshal entry
	data, err := proto.Marshal(entry)
	if err != nil {
		return err
	}
	// write chunksize
	if err := binary.Write(w.bufWriter, binary.LittleEndian, int32(len(data))); err != nil {
		return err
	}
	// write data
	byteSize, err := w.bufWriter.Write(data)
	if err != nil {
		return err
	}
	w.bufWriter.Flush()
	// update metadata
	w.curIndex += binary.Size(int32(len(data))) + byteSize
	w.metadata = append(w.metadata, w.curIndex)
	return nil
}

func (w *WAL) TruncateAt(index int32) error {
	// delete entry from WAL
	w.mu.Lock()
	defer w.mu.Unlock()
	// find the start of the entry
	start := w.metadata[index]
	// delete all bytes from the file after start
	if err := w.file.Truncate(int64(start)); err != nil {
		log.Printf("Failed to truncate file: %v\n", err)
		return err
	}

	_, err := w.file.Seek(0, io.SeekEnd)
	if err != nil {
		log.Printf("Failed to seek to end of file: %v\n", err)
		return err
	}

	// Reset the buffered writer with the new file position
	w.bufWriter.Reset(w.file)

	// TODO: FIX
	// update metadata
	w.metadata = w.metadata[:index]
	w.curIndex = start
	return nil
}

// TOOD: should this be refactored to use it's own file?
func (w *WAL) readAllEntries(file *os.File) ([]*pb.LogEntry, error) {
	// read all entries from file
	var entries []*pb.LogEntry
	for {
		var chunksize int32
		// break at end of file or some error
		if err := binary.Read(file, binary.LittleEndian, &chunksize); err != nil {
			if err == io.EOF {
				break
			}
			return entries, err
		}
		// read chunksize bytes
		data := make([]byte, chunksize)
		if _, err := io.ReadFull(file, data); err != nil {
			return entries, err
		}
		// decode data
		entry := &pb.LogEntry{}
		if err := proto.Unmarshal(data, entry); err != nil {
			log.Panic("failed to unmarshal entry", err)
		}

		entries = append(entries, entry)
		w.metadata = append(w.metadata, w.curIndex)
	}
	return entries, nil
}

// first pass for reading and writing everything
func (r *RaftLog) SaveLog() {
	// save log to disk
	file, err := os.OpenFile(r.wal.filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// write to file
	for _, entry := range r.entries {
		// write entry to file
		_, err := file.WriteString(entry.String())
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (r *RaftLog) LoadLog() {
	// load log from disk
	file, err := os.Open(r.wal.filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	r.entries, err = r.wal.readAllEntries(file)
	if err != nil {
		log.Fatal(err)
	}
}

func (r *RaftLog) Close() {
	r.wal.mu.Lock()
	defer r.wal.mu.Unlock()
	r.SaveLog()
}

func (r *RaftLog) Open() {
	r.wal.mu.Lock()
	defer r.wal.mu.Unlock()
	r.LoadLog()
}
