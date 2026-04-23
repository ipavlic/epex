package interpreter

// callLimitsMethod handles Limits.* static method calls.
// Returns sensible defaults for a test environment.
func callLimitsMethod(method string, args []*Value) (*Value, bool) {
	switch method {
	case "getqueries":
		return IntegerValue(0), true
	case "getlimitqueries":
		return IntegerValue(100), true
	case "getdmlstatements":
		return IntegerValue(0), true
	case "getlimitdmlstatements":
		return IntegerValue(150), true
	case "getdmlrows":
		return IntegerValue(0), true
	case "getlimitdmlrows":
		return IntegerValue(10000), true
	case "getheapsize":
		return IntegerValue(0), true
	case "getlimitheapsize":
		return IntegerValue(6000000), true
	case "getcputime":
		return IntegerValue(0), true
	case "getlimitcputime":
		return IntegerValue(10000), true
	case "getcallouts":
		return IntegerValue(0), true
	case "getlimitcallouts":
		return IntegerValue(100), true
	case "getfuturecalls":
		return IntegerValue(0), true
	case "getlimitfuturecalls":
		return IntegerValue(50), true
	case "getqueryrows":
		return IntegerValue(0), true
	case "getlimitqueryrows":
		return IntegerValue(50000), true
	case "getsosl", "getsoslqueries":
		return IntegerValue(0), true
	case "getlimitsosl", "getlimitsoslqueries":
		return IntegerValue(20), true
	case "getemailinvocations":
		return IntegerValue(0), true
	case "getlimitemailinvocations":
		return IntegerValue(10), true
	case "getqueueablejobs":
		return IntegerValue(0), true
	case "getlimitqueueablejobs":
		return IntegerValue(50), true
	case "getmobilepushapexcalls":
		return IntegerValue(0), true
	case "getlimitmobilepushapexcalls":
		return IntegerValue(10), true
	case "getpublishimmediatedml":
		return IntegerValue(0), true
	case "getlimitpublishimmediatedml":
		return IntegerValue(150), true
	}
	return nil, false
}
