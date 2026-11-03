// Package obdsnapshot publishes a consistent, fixed-size view of the PID
// cache to the UI. Producers write the inactive buffer; the UI reads the
// active one without locks.
package obdsnapshot

import (
	"sync"
	"sync/atomic"

	"m5stick-obd2-cnx/internal/obdpid"
)

const (
	// Slots bounds the snapshot to the same 64 PIDs as obdstore.
	Slots = 64
	// buffers gives the UI two publish cycles to finish a frame. With 3
	// buffers the producer never reuses the buffer being read.
	buffers = 3
)

// Slot is packed to 16 bytes: float64 (8) + uint32 (4) + uint16 (2) +
// uint8 (1) + 1 byte padding. Keep this order; it is part of the memory
// budget.
type Slot struct {
	Value   float64
	AtMs    uint32
	PID     uint16
	Quality obdpid.Quality
	_       uint8
}

// Snapshot is an immutable view once published. Slots are only touched by
// the producer while the snapshot is inactive.
type Snapshot struct {
	AtMs  uint32
	slots [Slots]Slot
}

// Get finds a PID by its two-byte ASCII key.
func (s *Snapshot) Get(key uint16) (Slot, bool) {
	if key == 0 {
		return Slot{}, false
	}
	for i := 0; i < Slots; i++ {
		if s.slots[i].PID == key {
			return s.slots[i], true
		}
	}
	return Slot{}, false
}

// Put inserts or updates a slot in the inactive snapshot.
func (s *Snapshot) Put(key uint16, value float64, q obdpid.Quality, atMs uint32) {
	if key == 0 {
		return
	}
	empty := -1
	for i := 0; i < Slots; i++ {
		slot := &s.slots[i]
		if slot.PID == key {
			slot.Value = value
			slot.AtMs = atMs
			slot.Quality = q
			return
		}
		if slot.PID == 0 && empty < 0 {
			empty = i
		}
	}
	if empty >= 0 {
		s.slots[empty] = Slot{Value: value, AtMs: atMs, PID: key, Quality: q}
	}
}

// Clear resets a snapshot without allocating.
func (s *Snapshot) Clear() {
	s.AtMs = 0
	for i := range s.slots {
		s.slots[i] = Slot{}
	}
}

// Store owns the rotating snapshots.
type Store struct {
	mu       sync.Mutex
	active   uint32
	writable uint32
	bufs     [buffers]Snapshot
}

func New() *Store { return &Store{} }

// Current returns the snapshot the UI may read. It is lock-free.
func (s *Store) Current() *Snapshot {
	return &s.bufs[atomic.LoadUint32(&s.active)]
}

// Begin returns the next writable buffer, pre-filled with the active values
// so PIDs that were not updated keep their last reading.
func (s *Store) Begin() *Snapshot {
	s.mu.Lock()
	next := (atomic.LoadUint32(&s.active) + 1) % buffers
	s.bufs[next] = *s.Current()
	s.writable = next
	s.mu.Unlock()
	return &s.bufs[next]
}

// Publish makes the writable buffer visible to the UI.
func (s *Store) Publish(atMs uint32) {
	s.mu.Lock()
	s.bufs[s.writable].AtMs = atMs
	atomic.StoreUint32(&s.active, s.writable)
	s.mu.Unlock()
}

// Clear drops every buffer. Used when replay resets the cache.
func (s *Store) Clear() {
	s.mu.Lock()
	for i := range s.bufs {
		s.bufs[i].Clear()
	}
	atomic.StoreUint32(&s.active, 0)
	s.writable = 0
	s.mu.Unlock()
}
