// Package finger2 drives the M5Stack Unit Fingerprint2 (A-K323CP) over UART.
//
// The implementation is pure Go: it does not use C, C++, CGo, Arduino or the
// ESP-IDF. Only machine.UART handles the 115200 8N1 Grove link (TX=G32,
// RX=G33 on the M5StickC Plus2).
//
// # Protocol
//
// Every packet is framed as:
//
//	EF 01 | address(4) | PID | length(2) | payload | checksum(2)
//
// where PID 0x01 is a command, 0x07 an ACK, 0x02 a data packet and 0x08 the
// final data packet. Only ACK payloads start with a confirmation byte; data
// packets carry raw data from byte 9 onwards. Mixing the two up silently
// drops one byte per data packet and corrupts image transfers.
//
// # Reading a finger
//
// Search compares the feature buffer, not the raw image, so reads must run:
//
//	Capture -> GenChar(1) -> Search(1, ...)
//
// Enrollment is delegated to the module's own state machine through
// StartAutoEnroll/NextEnrollEvent.
//
// # Concurrency
//
// A Device is safe for concurrent use: one-shot operations are serialized with
// a blocking lock. Auto-enroll and auto-identify sessions keep the device until
// the terminal stage, Cancel or context expiry; use the *Context variants to
// bound the wait. Every session is capped at 60 seconds.
package finger2
