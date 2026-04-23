package interpreter

import (
	"math"
	"strings"
)

// callDecimalInstanceMethod handles Decimal/Double/Integer instance methods beyond basic numeric.
func callDecimalInstanceMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	f, _ := obj.toFloat64()

	switch method {
	case "abs":
		if obj.Type == TypeInteger {
			i := obj.Data.(int)
			if i < 0 {
				i = -i
			}
			return IntegerValue(i), true
		}
		return DoubleValue(math.Abs(f)), true
	case "setscale":
		if len(args) >= 1 {
			scale, _ := args[0].toInt()
			factor := math.Pow(10, float64(scale))
			// Default rounding mode is HALF_UP
			if len(args) >= 2 {
				mode := args[1].ToString()
				return DoubleValue(roundWithMode(f, factor, mode)), true
			}
			return DoubleValue(math.Round(f*factor) / factor), true
		}
		return DoubleValue(f), true
	case "scale":
		// Approximate scale detection
		s := obj.ToString()
		dotIdx := -1
		for i, c := range s {
			if c == '.' {
				dotIdx = i
				break
			}
		}
		if dotIdx < 0 {
			return IntegerValue(0), true
		}
		return IntegerValue(len(s) - dotIdx - 1), true
	case "precision":
		s := obj.ToString()
		count := 0
		for _, c := range s {
			if c >= '0' && c <= '9' {
				count++
			}
		}
		return IntegerValue(count), true
	case "striptrailingzeros":
		if obj.Type == TypeInteger {
			return obj, true
		}
		// Remove trailing zeros
		s := obj.ToString()
		if dotIdx := strings.IndexByte(s, '.'); dotIdx >= 0 {
			end := len(s)
			for end > dotIdx+1 && s[end-1] == '0' {
				end--
			}
			if end == dotIdx+1 {
				end = dotIdx // remove dot too
			}
			s = s[:end]
		}
		return StringValue(s), true // will be used as-is or parsed
	case "round":
		return LongValue(int64(math.Round(f))), true
	case "pow":
		if len(args) >= 1 {
			exp, _ := args[0].toFloat64()
			return DoubleValue(math.Pow(f, exp)), true
		}
		return DoubleValue(f), true
	case "divide":
		if len(args) >= 2 {
			divisor, _ := args[0].toFloat64()
			scale, _ := args[1].toInt()
			if divisor == 0 {
				return NullValue(), true
			}
			result := f / divisor
			factor := math.Pow(10, float64(scale))
			return DoubleValue(math.Round(result*factor) / factor), true
		}
		return NullValue(), true
	case "min":
		if len(args) >= 1 {
			other, _ := args[0].toFloat64()
			return DoubleValue(math.Min(f, other)), true
		}
		return DoubleValue(f), true
	case "max":
		if len(args) >= 1 {
			other, _ := args[0].toFloat64()
			return DoubleValue(math.Max(f, other)), true
		}
		return DoubleValue(f), true
	case "toplaindouble":
		return DoubleValue(f), true
	case "toplainstring":
		return StringValue(obj.ToString()), true
	}
	return nil, false
}

func roundWithMode(f, factor float64, mode string) float64 {
	switch mode {
	case "HALF_UP":
		return math.Round(f*factor) / factor
	case "HALF_DOWN":
		// Round half toward zero
		shifted := f * factor
		if shifted-math.Floor(shifted) == 0.5 {
			return math.Floor(shifted) / factor
		}
		return math.Round(shifted) / factor
	case "CEILING", "UP":
		return math.Ceil(f*factor) / factor
	case "FLOOR", "DOWN":
		return math.Floor(f*factor) / factor
	default:
		return math.Round(f*factor) / factor
	}
}

