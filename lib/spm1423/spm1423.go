// Package spm1423 implements a PDM RX driver for the SPM1423 microphone
// found on the M5StickC Plus2, using the ESP32 classic I2S0 peripheral.
//
// The TinyGo machine package does not implement machine.I2S on the classic
// ESP32, so this package talks to the I2S0 registers directly. The ESP32 I2S
// block can convert PDM to PCM in hardware (pdm2pcm_conv_en), which keeps the
// Go side simple: we only read 16-bit PCM samples from a DMA buffer.
package spm1423

import (
	"errors"
	"math"
	"time"
	"unsafe"

	"device/esp"
	"machine"
)

const (
	// GPIO matrix signal indices from the ESP32 gpio_sig_map.h.
	sigI2S0RxWSOut  = 28  // I2S0I_WS_OUT: PDM clock output
	sigI2S0RxDataIn = 155 // I2S0I_DATA_IN15: PDM data input

	// 160 MHz PLL_D2 / 20 = 8 MHz MCLK; / 8 = 1 MHz BCLK.
	// PDM sample rate = 1 MHz / 64 = 15.625 kHz.
	mclkDivNum = 20
	mclkDivB   = 0
	mclkDivA   = 1
	bclkDiv    = 8

	ownerDMA = 1 << 31
	sucEOF   = 1 << 30
)

var (
	ErrTimeout = errors.New("spm1423: DMA timeout")
	ErrNoData  = errors.New("spm1423: no data")
)

// Config controls the microphone configuration.
type Config struct {
	// BufferSize is the number of PCM samples per DMA buffer.
	// Defaults to 512 (about 32 ms at 15.625 kHz).
	BufferSize int
}

// Device is an SPM1423 PDM microphone.
type Device struct {
	clockPin machine.Pin
	dataPin  machine.Pin
	i2s      *esp.I2S_Type

	buf     []byte
	desc    *dmaDescriptor
	addr    uint32
	started bool
}

// dmaDescriptor mirrors the 12-byte ESP32 DMA descriptor used by I2S.
// Word 0: size[0:11], length[12:23], err_eof[28], suc_eof[30], owner[31].
type dmaDescriptor struct {
	dw0  uint32
	buf  unsafe.Pointer
	next *dmaDescriptor
}

// New creates an SPM1423 device. On the M5StickC Plus2 use GPIO0 (clock)
// and GPIO34 (data).
func New(clockPin, dataPin machine.Pin) *Device {
	return &Device{
		clockPin: clockPin,
		dataPin:  dataPin,
		i2s:      esp.I2S0,
	}
}

// Configure sets up the GPIO matrix, I2S0 PDM RX mode and the DMA descriptor.
func (d *Device) Configure(cfg Config) error {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 512
	}
	if cfg.BufferSize > 4095/2 {
		cfg.BufferSize = 4095 / 2
	}

	d.configurePins()
	d.configureI2S()

	d.buf = make([]byte, cfg.BufferSize*2)
	d.i2s.RXEOF_NUM.Set(uint32(len(d.buf) / 4))
	d.desc = &dmaDescriptor{
		dw0: uint32(len(d.buf)) | sucEOF | ownerDMA,
		buf: unsafe.Pointer(&d.buf[0]),
	}
	d.addr = uint32(uintptr(unsafe.Pointer(d.desc))) & 0xFFFFF

	return nil
}

func (d *Device) configurePins() {
	// PDM clock: GPIO matrix output signal 28 (I2S0I_WS_OUT).
	d.clockPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	esp.GPIO.SetFUNC0_OUT_SEL_CFG_OUT_SEL(sigI2S0RxWSOut)
	esp.GPIO.SetFUNC0_OUT_SEL_CFG_OEN_SEL(0)

	// PDM data: GPIO34 drives input signal 155 (I2S0I_DATA_IN15).
	d.dataPin.Configure(machine.PinConfig{Mode: machine.PinInput})
	esp.GPIO.SetFUNC155_IN_SEL_CFG_IN_SEL(uint32(d.dataPin))
	esp.GPIO.SetFUNC155_IN_SEL_CFG_SEL(1)
}

func (d *Device) configureI2S() {
	// Enable and reset the I2S0 bus clock.
	esp.DPORT.SetPERIP_CLK_EN_I2S0_CLK_EN(1)
	esp.DPORT.SetPERIP_RST_EN_I2S0_RST(1)
	esp.DPORT.SetPERIP_RST_EN_I2S0_RST(0)

	i2s := d.i2s

	// Reset RX and FIFO.
	i2s.SetCONF_RX_RESET(1)
	i2s.SetCONF_RX_RESET(0)
	i2s.SetCONF_RX_FIFO_RESET(1)
	i2s.SetCONF_RX_FIFO_RESET(0)
	i2s.SetLC_CONF_IN_RST(1)
	i2s.SetLC_CONF_IN_RST(0)

	// Clock: PLL_D2 160 MHz / 20 = 8 MHz, then /8 = 1 MHz BCLK.
	i2s.SetCLKM_CONF_CLKA_ENA(0)
	i2s.SetCLKM_CONF_CLKM_DIV_NUM(mclkDivNum)
	i2s.SetCLKM_CONF_CLKM_DIV_B(mclkDivB)
	i2s.SetCLKM_CONF_CLKM_DIV_A(mclkDivA)
	i2s.SetCLKM_CONF_CLK_EN(1)

	// 16-bit samples, 8-bit BCLK divider.
	i2s.SetSAMPLE_RATE_CONF_RX_BITS_MOD(16)
	i2s.SetSAMPLE_RATE_CONF_RX_BCK_DIV_NUM(bclkDiv)

	// Mono, 16-bit: rx_fifo_mod=1 and force-enable the FIFO module.
	i2s.SetFIFO_CONF_RX_FIFO_MOD(1)
	i2s.SetFIFO_CONF_RX_FIFO_MOD_FORCE_EN(1)
	i2s.SetFIFO_CONF_DSCR_EN(1)

	// PDM RX with hardware PDM-to-PCM conversion.
	i2s.CONF2.Set(0)
	i2s.SetPDM_CONF_RX_PDM_EN(1)
	i2s.SetPDM_CONF_PDM2PCM_CONV_EN(1)
	i2s.SetPDM_CONF_RX_PDM_SINC_DSR_16_EN(0)

	// Mono -> left slot.
	i2s.SetCONF_CHAN_RX_CHAN_MOD(2)

	// MSB/right-first settings used by ESP-IDF for ESP32 PDM RX.
	i2s.SetCONF_RX_MSB_RIGHT(0)
	i2s.SetCONF_RX_RIGHT_FIRST(0)
}

// Start begins PDM reception.
func (d *Device) Start() {
	d.desc.dw0 = uint32(len(d.buf)) | sucEOF | ownerDMA
	d.i2s.SetIN_LINK_INLINK_ADDR(d.addr)
	d.i2s.SetIN_LINK_INLINK_STOP(0)
	d.i2s.SetIN_LINK_INLINK_START(1)
	d.i2s.SetCONF_RX_START(1)
	d.started = true
}

// Stop stops PDM reception.
func (d *Device) Stop() {
	d.i2s.SetCONF_RX_START(0)
	d.i2s.SetIN_LINK_INLINK_STOP(1)
	d.started = false
}

// Read waits for the next DMA buffer and copies PCM samples into dst.
// It returns the number of samples copied.
func (d *Device) Read(dst []int16) (int, error) {
	if !d.started {
		return 0, ErrNoData
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for d.desc.dw0&ownerDMA != 0 {
		if time.Now().After(deadline) {
			return 0, ErrTimeout
		}
		time.Sleep(time.Millisecond)
	}

	length := int((d.desc.dw0 >> 12) & 0xFFF)
	n := length / 2
	if n > len(dst) {
		n = len(dst)
	}
	for i := 0; i < n; i++ {
		dst[i] = int16(uint16(d.buf[2*i]) | uint16(d.buf[2*i+1])<<8)
	}

	// Re-arm the same descriptor for the next transfer.
	d.desc.dw0 = uint32(len(d.buf)) | sucEOF | ownerDMA
	d.i2s.SetIN_LINK_INLINK_RESTART(1)
	d.i2s.SetIN_LINK_INLINK_ADDR(d.addr)
	d.i2s.SetIN_LINK_INLINK_START(1)

	return n, nil
}

// RMS returns the AC RMS of the samples (DC offset removed).
func RMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s)
	}
	mean := sum / float64(len(samples))

	var sq float64
	for _, s := range samples {
		v := float64(s) - mean
		sq += v * v
	}
	return math.Sqrt(sq / float64(len(samples)))
}

// DBFS converts an RMS value to dBFS relative to 16-bit full scale.
func DBFS(rms float64) float64 {
	if rms < 1 {
		rms = 1
	}
	return 20 * math.Log10(rms/32768.0)
}
