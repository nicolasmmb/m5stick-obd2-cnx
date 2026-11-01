# Fingerprint2 examples

Each folder is a standalone TinyGo program for the M5StickC Plus2 using
[`lib/finger2`](../lib/finger2).

## Wiring

- Fingerprint2 on the Grove port: **TX = G32**, **RX = G33**, UART1, 115200 8N1.
- The examples call `InitWithTimeout` and then `SetWorkMode(1)` to keep the
  module awake while they run.

## Build and flash

```bash
tinygo flash -target=m5stick-c -port=/dev/cu.usbserial-XXXX -size short -monitor ./examples/read
```

Or build all of them:

```bash
make examples
```

## Examples

| Folder | Shows |
|---|---|
| `read` | `Capture` → `GenChar` → `Search` (the correct read flow) |
| `enroll` | `FirstFreePage` + `StartAutoEnroll`/`NextEnrollEvent` |
| `identify` | `StartAutoIdentify`/`NextIdentifyEvent` |
| `image` | `UpImageTo` with a reused buffer + ASCII preview |
| `templates` | `LoadChar`, `UploadTemplateAuto`, `DownloadTemplateAuto`, `Store`, `Delete` |
| `match` | manual `GenChar`/`Match`, then `RegModel`/`Store`/`Delete` |
| `system` | firmware, SN, `ReadSysParams`, `GetRandom`, `CheckSensor`, work mode, sleep time, `SetMatchLevel`, LED |
| `notepad` | `WriteNotepad`/`ReadNotepad` round trip |
| `inf` | `ReadINF` (512 bytes) |
| `context` | `InitContext` and `StartAutoEnrollContext` cancelled by context |
| `concurrency` | several goroutines sharing one `Device` (commands are serialized) |

## Notes

- `templates` and `match` need at least one enrolled template; use the `enroll`
  example first.
- `match` creates a temporary template and deletes it at the end.
- `image` prints a 40×20 ASCII preview over serial.
- The examples are built with the `esp32` build tag; they do not run on a host.
