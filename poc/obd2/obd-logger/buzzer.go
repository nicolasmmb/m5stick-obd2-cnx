package main

import (
	"machine"
	"time"
)

const (
	buzzerFreq = 2700
	beepMillis = 120
	maxBackoff = 10 * time.Second
	beepPeriod = 1 * time.Second
)

var (
	buzzerPWM = machine.PWM0
	buzzerCh  uint8
	buzzerOK  bool
)

// initBuzzer configures the passive buzzer on GPIO2 through LEDC PWM. The
// volume is fixed by the hardware; Top()/2 is the maximum square-wave drive.
func initBuzzer() {
	if err := buzzerPWM.Configure(machine.PWMConfig{Period: 1_000_000_000 / buzzerFreq}); err != nil {
		println("buzzer:", err.Error())
		return
	}
	ch, err := buzzerPWM.Channel(machine.GPIO2)
	if err != nil {
		println("buzzer:", err.Error())
		return
	}
	buzzerCh = ch
	buzzerOK = true
}

func beep(d time.Duration) {
	if !buzzerOK {
		time.Sleep(d)
		return
	}
	buzzerPWM.Set(buzzerCh, buzzerPWM.Top()/2)
	time.Sleep(d)
	buzzerPWM.Set(buzzerCh, 0)
}

// waitBackoff blocks for d (capped at 10 s), beeping once per second. The
// number of beeps therefore matches the backoff duration: 1, 2, 5 or 10.
func waitBackoff(d time.Duration) {
	if d > maxBackoff {
		d = maxBackoff
	}
	if d <= 0 {
		return
	}
	if !buzzerOK {
		time.Sleep(d)
		return
	}
	steps := int((d + beepPeriod - 1) / beepPeriod)
	if steps < 1 {
		steps = 1
	}
	step := d / time.Duration(steps)
	for i := 0; i < steps; i++ {
		beep(beepMillis * time.Millisecond)
		if rest := step - beepMillis*time.Millisecond; rest > 0 {
			time.Sleep(rest)
		}
	}
}
