//go:build tinygo

package main

import (
	"runtime"
	"strconv"
)

// printMem answers C,MEM. GC first so "free" is the memory actually
// available, not uncollected garbage.
func printMem(tag string) {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	free := uint64(0)
	if m.HeapSys > m.HeapAlloc {
		free = m.HeapSys - m.HeapAlloc
	}
	println("M," + tag +
		",alloc=" + strconv.FormatUint(m.HeapAlloc, 10) +
		",free=" + strconv.FormatUint(free, 10) +
		",sys=" + strconv.FormatUint(m.HeapSys, 10) +
		",idle=" + strconv.FormatUint(m.HeapIdle, 10) +
		",total=" + strconv.FormatUint(m.TotalAlloc, 10) +
		",gc=" + strconv.FormatUint(uint64(m.NumGC), 10))
}
