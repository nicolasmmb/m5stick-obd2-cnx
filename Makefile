PORT ?= /dev/cu.usbserial-5A6C0570831
TARGET ?= m5stick-c
TINYGO ?= tinygo

.PHONY: build check hello wifi ble display obd obd-client obd-bench obd-tune obd-probe monitor clean

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

check:
	@for f in ./examples/hello/main.go ./examples/wifi-scan/main.go ./examples/ble-scan/main.go ./examples/display-scan/main.go ./poc/obd2/obd-probe/main.go ./poc/obd2/obd-client/main.go ./poc/obd2/obd-bench/main.go ./poc/obd2/obd-tune/main.go ./poc/obd2/obd-display/main.go; do \
		echo "gopls check $$f"; \
		./scripts/gopls-tinygo.sh check "$$f"; \
	done

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

monitor:
	$(TINYGO) monitor -port=$(PORT)

clean:
	rm -f /tmp/m5-*.bin
