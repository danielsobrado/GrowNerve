package integration

import (
	"strings"
)

// Unit describes how a provider's unit maps onto the farm's unit vocabulary.
type Unit struct {
	Canonical string
	Dimension string
	Convert   func(float64) float64
}

func identity(value float64) float64 { return value }

// CanonicalUnit maps a provider unit and the quantity it measures (a property
// or device-class hint such as "humidity") to the farm's vocabulary. Unknown
// units are kept as reported so nothing is silently rescaled; an empty unit
// becomes "1" (dimensionless) because every measurement needs one.
func CanonicalUnit(raw, quantity string) Unit {
	quantity = strings.ToLower(quantity)
	switch strings.TrimSpace(raw) {
	case "°C", "ºC", "C", "degC", "celsius":
		return Unit{"degC", "temperature", identity}
	case "°F", "ºF", "F", "degF", "fahrenheit":
		return Unit{"degC", "temperature", func(value float64) float64 { return (value - 32) * 5 / 9 }}
	case "K":
		return Unit{"degC", "temperature", func(value float64) float64 { return value - 273.15 }}
	case "%":
		if strings.Contains(quantity, "humidity") {
			return Unit{"%RH", "relative_humidity", identity}
		}
		return Unit{"%", "ratio", identity}
	case "%RH":
		return Unit{"%RH", "relative_humidity", identity}
	case "lx", "lux":
		return Unit{"lx", "illuminance", identity}
	case "ppm":
		return Unit{"ppm", "concentration", identity}
	case "ppb":
		return Unit{"ppb", "concentration", identity}
	case "µg/m³", "μg/m³", "ug/m3":
		return Unit{"ug/m3", "mass_concentration", identity}
	case "hPa", "mbar":
		return Unit{"hPa", "pressure", identity}
	case "kPa":
		return Unit{"hPa", "pressure", func(value float64) float64 { return value * 10 }}
	case "W":
		return Unit{"W", "power", identity}
	case "kW":
		return Unit{"W", "power", func(value float64) float64 { return value * 1000 }}
	case "kWh":
		return Unit{"kWh", "energy", identity}
	case "Wh":
		return Unit{"kWh", "energy", func(value float64) float64 { return value / 1000 }}
	case "V":
		return Unit{"V", "voltage", identity}
	case "mV":
		return Unit{"V", "voltage", func(value float64) float64 { return value / 1000 }}
	case "A":
		return Unit{"A", "current", identity}
	case "mA":
		return Unit{"A", "current", func(value float64) float64 { return value / 1000 }}
	case "L", "l":
		return Unit{"L", "volume", identity}
	case "mS/cm":
		return Unit{"mS/cm", "conductivity", identity}
	case "":
		if strings.Contains(quantity, "ph") {
			return Unit{"pH", "acidity", identity}
		}
		return Unit{"1", "", identity}
	}
	unit := strings.TrimSpace(raw)
	if len(unit) > 32 {
		unit = unit[:32]
	}
	return Unit{unit, "", identity}
}

// ConvertTo converts a canonical reading into a channel's unit. Readings are
// already canonical, so only equal units convert; anything else is refused
// rather than stored with the wrong meaning.
func ConvertTo(value float64, from, to string) (float64, bool) {
	if from == to {
		return value, true
	}
	return 0, false
}

// Slug turns a device or property name into a channel-key segment.
func Slug(value string) string {
	var builder strings.Builder
	separator := false
	for _, character := range strings.ToLower(value) {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			if separator && builder.Len() > 0 {
				builder.WriteByte('_')
			}
			builder.WriteRune(character)
			separator = false
		default:
			separator = true
		}
	}
	if builder.Len() == 0 {
		return "device"
	}
	return builder.String()
}
