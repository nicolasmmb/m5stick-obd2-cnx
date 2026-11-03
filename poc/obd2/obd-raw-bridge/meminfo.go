package main

import (
	"runtime"
	"strconv"
)

func printMem() {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	free := uint64(0)
	if m.HeapSys > m.HeapAlloc {
		free = m.HeapSys - m.HeapAlloc
	}
	emit("M," + strconv.FormatInt(nowMs(), 10) +
		",alloc=" + strconv.FormatUint(m.HeapAlloc, 10) +
		",free=" + strconv.FormatUint(free, 10) +
		",sys=" + strconv.FormatUint(m.HeapSys, 10) +
		",gc=" + strconv.FormatUint(uint64(m.NumGC), 10))
}
