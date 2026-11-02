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

type Cache struct {
	mu sync.RWMutex
	m  map[string]obdpid.Reading
}

func NewCache() *Cache {
	return &Cache{m: make(map[string]obdpid.Reading)}
}

func (c *Cache) Put(r obdpid.Reading) {
	c.mu.Lock()
	c.m[r.PID] = r
	c.mu.Unlock()
}

func (c *Cache) Get(pid string) (obdpid.Reading, bool) {
	c.mu.RLock()
	r, ok := c.m[pid]
	c.mu.RUnlock()
	return r, ok
}

func (c *Cache) Snapshot() map[string]obdpid.Reading {
	c.mu.RLock()
	out := make(map[string]obdpid.Reading, len(c.m))
	for k, v := range c.m {
		out[k] = v
	}
	c.mu.RUnlock()
	return out
}

func (c *Cache) Clear() {
	c.mu.Lock()
	c.m = make(map[string]obdpid.Reading)
	c.mu.Unlock()
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
