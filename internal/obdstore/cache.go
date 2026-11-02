package obdstore

import (
	"sync"
	"time"

	"m5stick-obd2-cnx/internal/obdpid"
)

type Store interface {
	Put(obdpid.Reading)
	Get(pid string) (obdpid.Reading, bool)
	Snapshot() map[string]obdpid.Reading
	Clear()
}

// cacheSlots bounds the cache: the firmware only tracks a few dozen PIDs.
// A fixed table keeps memory flat and avoids map allocations in the hot path.
const cacheSlots = 64

type cacheSlot struct {
	key  [2]byte
	n    uint8
	used bool
	r    obdpid.Reading
}

type Cache struct {
	mu    sync.RWMutex
	slots [cacheSlots]cacheSlot
}

func NewCache() *Cache {
	return &Cache{}
}

func (c *Cache) Put(r obdpid.Reading) {
	key, n := keyOf(r.PID)
	c.mu.Lock()
	s := c.slotFor(key, n, true)
	if s != nil {
		s.key = key
		s.n = n
		s.used = true
		s.r = r
	}
	c.mu.Unlock()
}

// PutBytes stores a reading whose PID arrives as two ASCII bytes, avoiding a
// string allocation on the replay path.
func (c *Cache) PutBytes(pid [2]byte, r obdpid.Reading) {
	c.mu.Lock()
	s := c.slotFor(pid, 2, true)
	if s != nil {
		s.key = pid
		s.n = 2
		s.used = true
		s.r = r
	}
	c.mu.Unlock()
}

func (c *Cache) PutQuality(pid string, q obdpid.Quality) {
	key, n := keyOf(pid)
	c.mu.Lock()
	s := c.slotFor(key, n, true)
	if s != nil {
		if !s.used {
			s.key = key
			s.n = n
			s.used = true
			s.r.PID = pid
		}
		s.r.Quality = q
		s.r.At = time.Now()
	}
	c.mu.Unlock()
}

func (c *Cache) PutQualityBytes(pid [2]byte, q obdpid.Quality) {
	c.mu.Lock()
	s := c.slotFor(pid, 2, true)
	if s != nil {
		if !s.used {
			s.key = pid
			s.n = 2
			s.used = true
		}
		s.r.Quality = q
		s.r.At = time.Now()
	}
	c.mu.Unlock()
}

func (c *Cache) Get(pid string) (obdpid.Reading, bool) {
	key, n := keyOf(pid)
	c.mu.RLock()
	s := c.slotFor(key, n, false)
	var r obdpid.Reading
	ok := s != nil && s.used
	if ok {
		r = s.r
	}
	c.mu.RUnlock()
	return r, ok
}

func (c *Cache) Snapshot() map[string]obdpid.Reading {
	c.mu.RLock()
	out := make(map[string]obdpid.Reading, cacheSlots)
	for i := range c.slots {
		s := &c.slots[i]
		if s.used {
			out[s.r.PID] = s.r
		}
	}
	c.mu.RUnlock()
	return out
}

func (c *Cache) Clear() {
	c.mu.Lock()
	c.slots = [cacheSlots]cacheSlot{}
	c.mu.Unlock()
}

// slotFor returns the matching slot, or the first free one when create is
// true. It must be called with the lock held.
func (c *Cache) slotFor(key [2]byte, n uint8, create bool) *cacheSlot {
	start := (int(key[0])*31 + int(key[1])) & (cacheSlots - 1)
	firstFree := -1
	for i := 0; i < cacheSlots; i++ {
		s := &c.slots[(start+i)&(cacheSlots-1)]
		if s.used && s.n == n && s.key == key {
			return s
		}
		if !s.used && firstFree < 0 {
			firstFree = (start + i) & (cacheSlots - 1)
		}
	}
	if create && firstFree >= 0 {
		return &c.slots[firstFree]
	}
	return nil
}

func keyOf(pid string) ([2]byte, uint8) {
	var k [2]byte
	n := uint8(len(pid))
	if n > 2 {
		n = 2
	}
	for i := 0; i < int(n); i++ {
		k[i] = pid[i]
	}
	return k, n
}

type Snapshot struct {
	m  map[string]obdpid.Reading
	At time.Time
}

func NewSnapshot(m map[string]obdpid.Reading) Snapshot {
	return Snapshot{m: m, At: time.Now()}
}

func (s Snapshot) Get(pid string) (obdpid.Reading, bool) {
	r, ok := s.m[pid]
	if !ok {
		return obdpid.Reading{}, false
	}
	if r.Quality == obdpid.QualityOK && r.AgeMs(s.At) > 3000 {
		r.Quality = obdpid.QualityStale
	}
	return r, true
}
