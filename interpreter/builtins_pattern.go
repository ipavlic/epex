package interpreter

import (
	"regexp"
)

// callPatternStaticMethod handles Pattern.* static method calls.
func callPatternStaticMethod(method string, args []*Value) (*Value, bool) {
	switch method {
	case "compile":
		if len(args) >= 1 {
			pattern := args[0].ToString()
			// Store compiled regex as a string-typed value with SType for dispatch
			fields := map[string]*Value{
				"pattern": StringValue(pattern),
			}
			return &Value{Type: TypeSObject, Data: fields, SType: "Pattern"}, true
		}
		return NullValue(), true
	case "matches":
		if len(args) >= 2 {
			pattern := args[0].ToString()
			input := args[1].ToString()
			re, err := regexp.Compile(javaRegexToGo(pattern))
			if err != nil {
				return BooleanValue(false), true
			}
			return BooleanValue(re.MatchString(input)), true
		}
		return BooleanValue(false), true
	case "quote":
		if len(args) >= 1 {
			return StringValue(regexp.QuoteMeta(args[0].ToString())), true
		}
		return StringValue(""), true
	}
	return nil, false
}

// javaRegexToGo does minimal conversion of Java regex to Go regex.
func javaRegexToGo(pattern string) string {
	// Go's regexp doesn't support some Java regex features, but most basics work
	return pattern
}
