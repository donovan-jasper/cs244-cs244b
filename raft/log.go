package raftlog

import (
	"bufio"
	"encoding/binary"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
)

//accept and convert protobuf uint64o LogEntry struct
// convert LogEntry uint64o protobuf

// CALL 3 ENTRIES:
// append a LogEntry Protobuf (which will be a struct with protobuf functions)
// read a LogEntry (give an index, return)
// delete a LogEntry (give an index, return)

// creating raft log entry
// type LogEntry struct {
// 	term    uint64    // term when entry was received by leader
// 	index   uint64    // ensure ordering
// 	command string // change to bytes?
// }

// wal of https://github.com/JyotinderSingh/go-wal as reference
// assume file is int32 (file size), then the data itself
type WAL struct {
	filename  string
	mu        sync.RWMutex
	bufWriter *bufio.Writer
	syncTimer *time.Timer
}
type RaftLog struct {
	entries []*LogEntry
	wal     WAL
}

func NewWAL(filename string) *WAL {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	return &WAL{
		filename:  filename,
		bufWriter: bufio.NewWriter(file),
	}
}

func (r *RaftLog) GetEntry(index uint64) *LogEntry {
	return r.entries[index]
}

func (r *RaftLog) DeleteEntry(index uint64) {
	r.entries = append(r.entries[:index], r.entries[index+1:]...)
}

func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.bufWriter.Flush()
}

// testing
func (w *WAL) SaveLog() {
	// save log to disk
	file, err := os.OpenFile(w.filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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

// func (w *WAL) writeEntry(entry *LogEntry) {
// 	// write entry to file

// 	_, err := file.WriteString(entry.String())
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// }

func (w *WAL) readAllEntries(file *os.File) ([]*LogEntry, error) {
	// read all entries from file
	var entries []*LogEntry
	// entries := make([]LogEntry, 0)
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
		entry := &LogEntry{}
		if err := proto.Unmarshal(data, entry); err != nil {
			log.Panic("failed to unmarshal entry", err)
		}

		entries = append(entries, entry)
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
