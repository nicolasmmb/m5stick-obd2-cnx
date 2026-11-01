PORT ?= /dev/cu.usbserial-5A6C0570831
TARGET ?= m5stick-c
TINYGO ?= tinygo

.PHONY: build check hello encoder gps tof wifi ble display sensors mic env finger finger-test monitor clean

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
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-finger2-render-bench.bin ./poc/finger2-render-bench

check:
	@for f in ./poc/hello/main.go ./poc/wifi-scan/main.go ./poc/ble-scan/main.go ./poc/display-scan/main.go ./poc/sensors/main.go ./poc/mic-pdm/main.go ./poc/env-iv/main.go ./poc/finger2/main.go ./poc/finger2-test/main.go ./poc/finger2-bench/main.go ./poc/finger2-render-bench/main.go ./poc/encoder/main.go ./poc/gps/main.go ./poc/tof/main.go ./lib/spm1423/spm1423.go; do \
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

monitor:
	$(TINYGO) monitor -port=$(PORT)

clean:
	rm -f /tmp/m5-*.bin
