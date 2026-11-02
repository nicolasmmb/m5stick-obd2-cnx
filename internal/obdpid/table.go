package obdpid

// Table contains the PIDs used by the M5Stick logger and host decoder.
func Table() map[string]Decoder {
	return map[string]Decoder{
		"0C": U16{ID: "0C", Label: "Engine speed", UnitLabel: "rpm", Scale: 0.25},
		"0D": U8{ID: "0D", Label: "Vehicle speed", UnitLabel: "km/h", Scale: 1},
		"05": U8{ID: "05", Label: "Engine coolant temperature", UnitLabel: "C", Scale: 1, Offset: -40},
		"0F": U8{ID: "0F", Label: "Intake air temperature", UnitLabel: "C", Scale: 1, Offset: -40},
		"46": U8{ID: "46", Label: "Ambient air temperature", UnitLabel: "C", Scale: 1, Offset: -40},
		"11": U8{ID: "11", Label: "Throttle position", UnitLabel: "%", Scale: 100.0 / 255.0},
		"04": U8{ID: "04", Label: "Calculated engine load", UnitLabel: "%", Scale: 100.0 / 255.0},
		"2F": U8{ID: "2F", Label: "Fuel tank level", UnitLabel: "%", Scale: 100.0 / 255.0},
		"52": U8{ID: "52", Label: "Ethanol fuel percentage", UnitLabel: "%", Scale: 100.0 / 255.0},
		"45": U8{ID: "45", Label: "Relative throttle position", UnitLabel: "%", Scale: 100.0 / 255.0},
		"49": U8{ID: "49", Label: "Accelerator pedal D", UnitLabel: "%", Scale: 100.0 / 255.0},
		"4A": U8{ID: "4A", Label: "Accelerator pedal E", UnitLabel: "%", Scale: 100.0 / 255.0},
		"4C": U8{ID: "4C", Label: "Commanded throttle actuator", UnitLabel: "%", Scale: 100.0 / 255.0},
		"0B": U8{ID: "0B", Label: "Intake manifold pressure", UnitLabel: "kPa", Scale: 1},
		"33": U8{ID: "33", Label: "Barometric pressure", UnitLabel: "kPa", Scale: 1},
		"10": U16{ID: "10", Label: "MAF air flow rate", UnitLabel: "g/s", Scale: 0.01},
		"42": U16{ID: "42", Label: "Control module voltage", UnitLabel: "V", Scale: 0.001},
		"3C": U16{ID: "3C", Label: "Catalyst temperature B1S1", UnitLabel: "C", Scale: 0.1, Offset: -40},
		"44": U16{ID: "44", Label: "Commanded equivalence ratio", UnitLabel: "lambda", Scale: 2.0 / 65536.0},
		"06": Signed8{ID: "06", Label: "Short term fuel trim B1", UnitLabel: "%", Scale: 100.0 / 128.0},
		"07": Signed8{ID: "07", Label: "Long term fuel trim B1", UnitLabel: "%", Scale: 100.0 / 128.0},
		"0E": U8{ID: "0E", Label: "Timing advance", UnitLabel: "deg", Scale: 0.5, Offset: -64},
		"1F": U16{ID: "1F", Label: "Run time since engine start", UnitLabel: "s", Scale: 1},
		"21": U16{ID: "21", Label: "Distance with MIL on", UnitLabel: "km", Scale: 1},
		"31": U16{ID: "31", Label: "Distance since codes cleared", UnitLabel: "km", Scale: 1},
		"43": U16{ID: "43", Label: "Absolute load value", UnitLabel: "%", Scale: 100.0 / 255.0},
	}
}
