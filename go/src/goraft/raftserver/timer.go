package raftserver

import (
	"time"
)

type Callback func(rs *RaftServer)

type Timer struct {
	Duration time.Duration
	Server   *RaftServer
	Callback Callback
	stop     chan bool
	running  bool
}

// Function to create a new timer
func NewTimer(duration time.Duration, rs *RaftServer, callback Callback) *Timer {
	return &Timer{
		Duration: duration,
		Server:   rs,
		Callback: callback,
		stop:     make(chan bool),
		running:  false,
	}
}

func (t *Timer) Run() {
	t.running = true
	select {
	case <-time.After(t.Duration):
		t.running = false
		t.Callback(t.Server)
	case <-t.stop:
		t.running = false
	}
}

func (t *Timer) Stop() {
	if t.running {
		t.stop <- true
	}
}
