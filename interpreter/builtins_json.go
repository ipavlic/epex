package interpreter

import (
	"encoding/json"
	"strings"
)

// callJSONMethod handles JSON.* static method calls.
func callJSONMethod(method string, args []*Value) (*Value, bool) {
	switch method {
	case "serialize":
		if len(args) >= 1 {
			return StringValue(serializeValue(args[0])), true
		}
		return StringValue("null"), true
	case "serializepretty":
		if len(args) >= 1 {
			return StringValue(serializeValuePretty(args[0])), true
		}
		return StringValue("null"), true
	case "deserialize", "deserializestrict":
		if len(args) >= 1 {
			s := args[0].ToString()
			return deserializeJSON(s), true
		}
		return NullValue(), true
	case "deserializeuntyped":
		if len(args) >= 1 {
			s := args[0].ToString()
			return deserializeJSON(s), true
		}
		return NullValue(), true
	case "createparser":
		// Simplified: just return the string for now
		if len(args) >= 1 {
			return args[0], true
		}
		return NullValue(), true
	case "creategenerator":
		// Simplified: return empty string builder
		return StringValue(""), true
	}
	return nil, false
}

func serializeValue(v *Value) string {
	goVal := valueToGo(v)
	b, err := json.Marshal(goVal)
	if err != nil {
		return "null"
	}
	return string(b)
}

func serializeValuePretty(v *Value) string {
	goVal := valueToGo(v)
	b, err := json.MarshalIndent(goVal, "", "  ")
	if err != nil {
		return "null"
	}
	return string(b)
}

func valueToGo(v *Value) any {
	if v == nil || v.Type == TypeNull {
		return nil
	}
	switch v.Type {
	case TypeBoolean:
		return v.Data.(bool)
	case TypeInteger:
		return v.Data.(int)
	case TypeLong:
		return v.Data.(int64)
	case TypeDouble:
		return v.Data.(float64)
	case TypeString:
		return v.Data.(string)
	case TypeList:
		elements := v.Data.([]*Value)
		result := make([]any, len(elements))
		for i, e := range elements {
			result[i] = valueToGo(e)
		}
		return result
	case TypeMap, TypeSObject:
		entries := v.Data.(map[string]*Value)
		result := make(map[string]any, len(entries))
		for k, val := range entries {
			if !strings.HasPrefix(k, "_") { // skip internal fields
				result[k] = valueToGo(val)
			}
		}
		return result
	case TypeSet:
		entries := v.Data.(map[string]*Value)
		result := make([]any, 0, len(entries))
		for k := range entries {
			result = append(result, k)
		}
		return result
	}
	return v.ToString()
}

func deserializeJSON(s string) *Value {
	var raw any
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return NullValue()
	}
	return goToValue(raw)
}

func goToValue(v any) *Value {
	if v == nil {
		return NullValue()
	}
	switch tv := v.(type) {
	case bool:
		return BooleanValue(tv)
	case float64:
		// JSON numbers are always float64
		if tv == float64(int(tv)) && tv >= -2147483648 && tv <= 2147483647 {
			return IntegerValue(int(tv))
		}
		return DoubleValue(tv)
	case string:
		return StringValue(tv)
	case []any:
		elements := make([]*Value, len(tv))
		for i, e := range tv {
			elements[i] = goToValue(e)
		}
		return ListValue(elements)
	case map[string]any:
		entries := make(map[string]*Value, len(tv))
		for k, val := range tv {
			entries[k] = goToValue(val)
		}
		return MapValue(entries)
	}
	return NullValue()
}
