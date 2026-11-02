PORT ?= /dev/cu.usbserial-5A6C0570831
TARGET ?= m5stick-c
TINYGO ?= tinygo

.PHONY: build check hello encoder gps tof joystick2 nfc wifi ble display sensors mic env finger finger-test obd obd-client obd-bench obd-tune obd-probe monitor clean

build:
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-hello.bin ./poc/hello
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-wifi-scan.bin ./poc/wifi-scan
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-ble-scan.bin ./poc/ble-scan
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-display-scan.bin ./poc/display-scan
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-sensors.bin ./poc/sensors
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-mic-pdm.bin ./poc/mic-pdm
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-env-iv.bin ./poc/env-iv
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-finger2.bin ./poc/finger2
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-finger2-test.bin ./poc/finger2-test
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-finger2-bench.bin ./poc/finger2-bench
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-encoder.bin ./poc/encoder
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-gps.bin ./poc/gps
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-tof.bin ./poc/tof
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-joystick2.bin ./poc/joystick2
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-nfc.bin ./poc/nfc
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-finger2-render-bench.bin ./poc/finger2-render-bench
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-obd-probe.bin ./poc/obd-probe
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-obd-client.bin ./poc/obd-client
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-obd-bench.bin ./poc/obd-bench
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-obd-tune.bin ./poc/obd-tune
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-obd-display.bin ./poc/obd-display

check:
	@for f in ./poc/hello/main.go ./poc/wifi-scan/main.go ./poc/ble-scan/main.go ./poc/display-scan/main.go ./poc/sensors/main.go ./poc/mic-pdm/main.go ./poc/env-iv/main.go ./poc/finger2/main.go ./poc/finger2-test/main.go ./poc/finger2-bench/main.go ./poc/finger2-render-bench/main.go ./poc/encoder/main.go ./poc/gps/main.go ./poc/tof/main.go ./poc/joystick2/main.go ./poc/nfc/main.go ./poc/obd-probe/main.go ./poc/obd-client/main.go ./poc/obd-bench/main.go ./poc/obd-tune/main.go ./poc/obd-display/main.go ./lib/spm1423/spm1423.go; do \
		echo "gopls check $$f"; \
		./scripts/gopls-tinygo.sh check "$$f"; \
	done

hello:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/hello

wifi:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/wifi-scan

ble:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/ble-scan

display:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/display-scan

sensors:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/sensors

mic:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/mic-pdm

env:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/env-iv

finger:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/finger2

finger-test:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/finger2-test

encoder:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/encoder

gps:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/gps

tof:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/tof

joystick2:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/joystick2

nfc:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/nfc

obd:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/obd-display

obd-probe:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/obd-probe

obd-client:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/obd-client

obd-bench:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/obd-bench

obd-tune:
	$(TINYGO) flash -target=$(TARGET) -port=$(PORT) -size short -monitor ./poc/obd-tune

monitor:
	$(TINYGO) monitor -port=$(PORT)

clean:
	rm -f /tmp/m5-*.bin
