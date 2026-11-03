package main

import "sync/atomic"

// byteRing is a fixed single-producer/single-consumer ring. It never
// allocates; if full, the producer drops the new bytes.
type byteRing struct {
	buf     [2048]byte
	head    uint32
	tail    uint32
	dropped uint32
}

func (r *byteRing) write(p []byte) int {
	n := 0
	for _, b := range p {
		h := atomic.LoadUint32(&r.head)
		next := (h + 1) % uint32(len(r.buf))
		if next == atomic.LoadUint32(&r.tail) {
			atomic.AddUint32(&r.dropped, 1)
			break
		}
		r.buf[h] = b
		atomic.StoreUint32(&r.head, next)
		n++
	}
	return n
}

func (r *byteRing) read(p []byte) int {
	n := 0
	for n < len(p) {
		t := atomic.LoadUint32(&r.tail)
		if t == atomic.LoadUint32(&r.head) {
			break
		}
		p[n] = r.buf[t]
		atomic.StoreUint32(&r.tail, (t+1)%uint32(len(r.buf)))
		n++
	}
	return n
}

func (r *byteRing) dropCount() uint32 {
	return atomic.LoadUint32(&r.dropped)
}
