package interpreter

import (
	"math"
	"math/rand"
)

// callMathMethod handles Math.* static method calls.
func callMathMethod(method string, args []*Value) (*Value, bool) {
	switch method {
	case "abs":
		if len(args) >= 1 {
			switch args[0].Type {
			case TypeInteger:
				v := args[0].Data.(int)
				if v < 0 {
					v = -v
				}
				return IntegerValue(v), true
			case TypeLong:
				v := args[0].Data.(int64)
				if v < 0 {
					v = -v
				}
				return LongValue(v), true
			case TypeDouble:
				return DoubleValue(math.Abs(args[0].Data.(float64))), true
			}
		}
		return NullValue(), true
	case "acos":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Acos(f)), true
		}
		return NullValue(), true
	case "asin":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Asin(f)), true
		}
		return NullValue(), true
	case "atan":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Atan(f)), true
		}
		return NullValue(), true
	case "atan2":
		if len(args) >= 2 {
			y, _ := args[0].toFloat64()
			x, _ := args[1].toFloat64()
			return DoubleValue(math.Atan2(y, x)), true
		}
		return NullValue(), true
	case "cbrt":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Cbrt(f)), true
		}
		return NullValue(), true
	case "ceil":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Ceil(f)), true
		}
		return NullValue(), true
	case "cos":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Cos(f)), true
		}
		return NullValue(), true
	case "cosh":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Cosh(f)), true
		}
		return NullValue(), true
	case "exp":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Exp(f)), true
		}
		return NullValue(), true
	case "floor":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Floor(f)), true
		}
		return NullValue(), true
	case "log":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Log(f)), true
		}
		return NullValue(), true
	case "log10":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Log10(f)), true
		}
		return NullValue(), true
	case "max":
		if len(args) >= 2 {
			if args[0].Type == TypeInteger && args[1].Type == TypeInteger {
				a := args[0].Data.(int)
				b := args[1].Data.(int)
				if a > b {
					return IntegerValue(a), true
				}
				return IntegerValue(b), true
			}
			a, _ := args[0].toFloat64()
			b, _ := args[1].toFloat64()
			return DoubleValue(math.Max(a, b)), true
		}
		return NullValue(), true
	case "min":
		if len(args) >= 2 {
			if args[0].Type == TypeInteger && args[1].Type == TypeInteger {
				a := args[0].Data.(int)
				b := args[1].Data.(int)
				if a < b {
					return IntegerValue(a), true
				}
				return IntegerValue(b), true
			}
			a, _ := args[0].toFloat64()
			b, _ := args[1].toFloat64()
			return DoubleValue(math.Min(a, b)), true
		}
		return NullValue(), true
	case "mod":
		if len(args) >= 2 {
			if args[0].Type == TypeInteger && args[1].Type == TypeInteger {
				b := args[1].Data.(int)
				if b == 0 {
					return NullValue(), true
				}
				return IntegerValue(args[0].Data.(int) % b), true
			}
			a, _ := args[0].toFloat64()
			b, _ := args[1].toFloat64()
			if b == 0 {
				return NullValue(), true
			}
			return DoubleValue(math.Mod(a, b)), true
		}
		return NullValue(), true
	case "pow":
		if len(args) >= 2 {
			base, _ := args[0].toFloat64()
			exp, _ := args[1].toFloat64()
			return DoubleValue(math.Pow(base, exp)), true
		}
		return NullValue(), true
	case "random":
		return DoubleValue(rand.Float64()), true
	case "rint":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.RoundToEven(f)), true
		}
		return NullValue(), true
	case "round":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return LongValue(int64(math.Round(f))), true
		}
		return NullValue(), true
	case "roundup":
		if len(args) >= 2 {
			f, _ := args[0].toFloat64()
			scale, _ := args[1].toInt()
			factor := math.Pow(10, float64(scale))
			return DoubleValue(math.Ceil(f*factor) / factor), true
		}
		return NullValue(), true
	case "rounddown":
		if len(args) >= 2 {
			f, _ := args[0].toFloat64()
			scale, _ := args[1].toInt()
			factor := math.Pow(10, float64(scale))
			return DoubleValue(math.Floor(f*factor) / factor), true
		}
		return NullValue(), true
	case "signum":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			if f > 0 {
				return DoubleValue(1.0), true
			} else if f < 0 {
				return DoubleValue(-1.0), true
			}
			return DoubleValue(0.0), true
		}
		return NullValue(), true
	case "sin":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Sin(f)), true
		}
		return NullValue(), true
	case "sinh":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Sinh(f)), true
		}
		return NullValue(), true
	case "sqrt":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Sqrt(f)), true
		}
		return NullValue(), true
	case "tan":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Tan(f)), true
		}
		return NullValue(), true
	case "tanh":
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DoubleValue(math.Tanh(f)), true
		}
		return NullValue(), true
	}
	return nil, false
}
