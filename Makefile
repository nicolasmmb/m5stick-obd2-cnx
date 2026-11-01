PORT ?= /dev/cu.usbserial-5A6C0570831
TARGET ?= m5stick-c
TINYGO ?= tinygo

.PHONY: build check hello wifi ble display monitor clean

build:
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-hello.bin ./poc/hello
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-wifi-scan.bin ./poc/wifi-scan
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-ble-scan.bin ./poc/ble-scan
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5-display-scan.bin ./poc/display-scan

check:
	@for f in ./poc/hello/main.go ./poc/wifi-scan/main.go ./poc/ble-scan/main.go ./poc/display-scan/main.go; do \
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

monitor:
	$(TINYGO) monitor -port=$(PORT)

clean:
	rm -f /tmp/m5-*.bin
