PORT ?= /dev/cu.usbserial-5A6C0559481
TARGET ?= m5stick-c
TINYGO ?= tinygo
FILE ?= testdata/obd-logger-sample-compact.txt
SPEED ?= 1
DROP_AFTER ?= 0
ML_OUT ?= reports

.PHONY: build check test logger log replay replay-device sim ml ml-test hello wifi ble display obd obd-client obd-bench obd-tune obd-logger obd-probe monitor clean

build:
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-hello.bin ./examples/hello
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-wifi-scan.bin ./examples/wifi-scan
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-ble-scan.bin ./examples/ble-scan
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-display-scan.bin ./examples/display-scan
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-obd-probe.bin ./poc/obd2/obd-probe
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-obd-client.bin ./poc/obd2/obd-client
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-obd-bench.bin ./poc/obd2/obd-bench
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-obd-tune.bin ./poc/obd2/obd-tune
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-obd-display.bin ./poc/obd2/obd-display
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-obd-logger.bin ./poc/obd2/obd-logger

check:
	@for f in ./examples/hello/main.go ./examples/wifi-scan/main.go ./examples/ble-scan/main.go ./examples/display-scan/main.go ./poc/obd2/obd-probe/main.go ./poc/obd2/obd-client/main.go ./poc/obd2/obd-bench/main.go ./poc/obd2/obd-tune/main.go ./poc/obd2/obd-display/main.go ./poc/obd2/obd-logger/main.go; do \
		echo "gopls check $$f"; \
		./scripts/gopls-tinygo.sh check "$$f"; \
	done

test:
	go test ./internal/... ./cmd/... ./poc/obd2/obd-logger

logger:
	go build -o /tmp/obd-logger ./cmd/obd-logger

log: logger
	/tmp/obd-logger --port=$(PORT) --baud=115200 --out=data

replay: logger
	/tmp/obd-logger --replay=$(FILE) --out=data

replay-device:
	go run ./cmd/obd-replay --port=$(PORT) --file=$(FILE) --speed=$(SPEED)

ml:
	cd ml && uv run obdml --source data --data $(CURDIR)/data --out $(ML_OUT)

ml-test:
	cd ml && uv run pytest

sim:
	go run ./cmd/obd-replay --sim --port=$(PORT) --file=$(FILE) --drop-after=$(DROP_AFTER)

hello:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./examples/hello

wifi:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./examples/wifi-scan

ble:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./examples/ble-scan

display:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./examples/display-scan

obd:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/obd2/obd-display

obd-probe:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/obd2/obd-probe

obd-client:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/obd2/obd-client

obd-bench:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/obd2/obd-bench

obd-tune:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/obd2/obd-tune

obd-logger:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/obd2/obd-logger

monitor:
	$(TINYGO) monitor -port=$(PORT)

clean:
	rm -f /tmp/m5-*.bin
