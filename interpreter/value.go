package interpreter

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// NullPointerError is thrown when a null value is used in a numeric operation.
type NullPointerError struct {
	Message string
}

func (e *NullPointerError) Error() string {
	return "System.NullPointerException: " + e.Message
}

// ArithmeticError is thrown on division by zero.
type ArithmeticError struct {
	Message string
}

func (e *ArithmeticError) Error() string {
	return "System.MathException: " + e.Message
}

// panicNullPointer panics if either operand is null in a numeric context.
func panicNullPointer(v, other *Value) {
	if (v == nil || v.Type == TypeNull) || (other == nil || other.Type == TypeNull) {
		panic(&NullPointerError{Message: "Attempt to de-reference a null object"})
	}
}

// wrapInt32 truncates an int to 32-bit range, matching Apex's integer overflow behavior.
func wrapInt32(n int) int {
	return int(int32(n))
}

// ValueType represents the type of a runtime value.
type ValueType int

const (
	TypeNull ValueType = iota
	TypeBoolean
	TypeInteger
	TypeLong
	TypeDouble
	TypeString
	TypeSObject
	TypeList
	TypeSet
	TypeMap
)

// Value is the runtime value used throughout the interpreter.
type Value struct {
	Type    ValueType
	Data    any
	SType   string // for SObjects, the type name (e.g. "Account")
}

// NullValue returns a null value.
func NullValue() *Value {
	return &Value{Type: TypeNull, Data: nil}
}

// BooleanValue returns a boolean value.
func BooleanValue(b bool) *Value {
	return &Value{Type: TypeBoolean, Data: b}
}

// IntegerValue returns an integer value.
func IntegerValue(i int) *Value {
	return &Value{Type: TypeInteger, Data: i}
}

// LongValue returns a long value.
func LongValue(l int64) *Value {
	return &Value{Type: TypeLong, Data: l}
}

// DoubleValue returns a double value.
func DoubleValue(d float64) *Value {
	return &Value{Type: TypeDouble, Data: d}
}

// StringValue returns a string value.
func StringValue(s string) *Value {
	return &Value{Type: TypeString, Data: s}
}

// SObjectValue returns an SObject value.
func SObjectValue(typeName string, fields map[string]*Value) *Value {
	if fields == nil {
		fields = make(map[string]*Value)
	}
	return &Value{Type: TypeSObject, Data: fields, SType: typeName}
}

// ListValue returns a list value.
func ListValue(elements []*Value) *Value {
	if elements == nil {
		elements = []*Value{}
	}
	return &Value{Type: TypeList, Data: elements}
}

// SetValue returns a set value.
func SetValue(elements map[string]*Value) *Value {
	if elements == nil {
		elements = make(map[string]*Value)
	}
	return &Value{Type: TypeSet, Data: elements}
}

// MapValue returns a map value.
func MapValue(entries map[string]*Value) *Value {
	if entries == nil {
		entries = make(map[string]*Value)
	}
	return &Value{Type: TypeMap, Data: entries}
}

// IsTruthy returns true if the value is truthy.
func (v *Value) IsTruthy() bool {
	if v == nil || v.Type == TypeNull {
		return false
	}
	switch v.Type {
	case TypeBoolean:
		return v.Data.(bool)
	case TypeInteger:
		return v.Data.(int) != 0
	case TypeLong:
		return v.Data.(int64) != 0
	case TypeDouble:
		return v.Data.(float64) != 0
	case TypeString:
		return v.Data.(string) != ""
	case TypeList:
		return len(v.Data.([]*Value)) > 0
	case TypeMap, TypeSObject:
		return len(v.Data.(map[string]*Value)) > 0
	case TypeSet:
		return len(v.Data.(map[string]*Value)) > 0
	}
	return true
}

// ToString returns a string representation of the value.
func (v *Value) ToString() string {
	if v == nil || v.Type == TypeNull {
		return "null"
	}
	switch v.Type {
	case TypeBoolean:
		if v.Data.(bool) {
			return "true"
		}
		return "false"
	case TypeInteger:
		return strconv.Itoa(v.Data.(int))
	case TypeLong:
		return strconv.FormatInt(v.Data.(int64), 10)
	case TypeDouble:
		return fmt.Sprintf("%g", v.Data.(float64))
	case TypeString:
		return v.Data.(string)
	case TypeSObject:
		fields := v.Data.(map[string]*Value)
		var parts []string
		for k, val := range fields {
			parts = append(parts, k+"="+val.ToString())
		}
		return v.SType + ":{" + strings.Join(parts, ", ") + "}"
	case TypeList:
		elements := v.Data.([]*Value)
		parts := make([]string, len(elements))
		for i, e := range elements {
			parts[i] = e.ToString()
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case TypeMap:
		entries := v.Data.(map[string]*Value)
		var parts []string
		for k, val := range entries {
			parts = append(parts, k+"="+val.ToString())
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case TypeSet:
		entries := v.Data.(map[string]*Value)
		var parts []string
		for k := range entries {
			parts = append(parts, k)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return fmt.Sprintf("%v", v.Data)
}

// ToGoValue returns the underlying Go value.
func (v *Value) ToGoValue() any {
	if v == nil || v.Type == TypeNull {
		return nil
	}
	return v.Data
}

// toFloat64 converts a numeric value to float64 for arithmetic.
func (v *Value) toFloat64() (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch v.Type {
	case TypeInteger:
		return float64(v.Data.(int)), true
	case TypeLong:
		return float64(v.Data.(int64)), true
	case TypeDouble:
		return v.Data.(float64), true
	}
	return 0, false
}

// toInt returns the integer value if possible.
func (v *Value) toInt() (int, bool) {
	if v == nil {
		return 0, false
	}
	switch v.Type {
	case TypeInteger:
		return v.Data.(int), true
	case TypeLong:
		return int(v.Data.(int64)), true
	case TypeDouble:
		return int(v.Data.(float64)), true
	}
	return 0, false
}

// isNumeric returns true if the value is a number.
func (v *Value) isNumeric() bool {
	if v == nil {
		return false
	}
	return v.Type == TypeInteger || v.Type == TypeLong || v.Type == TypeDouble
}

// Add returns v + other.
func (v *Value) Add(other *Value) *Value {
	// String concatenation allows null (becomes "null")
	if (v != nil && v.Type == TypeString) || (other != nil && other.Type == TypeString) {
		return StringValue(safeVal(v).ToString() + safeVal(other).ToString())
	}
	panicNullPointer(v, other)
	// If either is double, result is double
	if v.Type == TypeDouble || other.Type == TypeDouble {
		a, _ := v.toFloat64()
		b, _ := other.toFloat64()
		return DoubleValue(a + b)
	}
	// If either is long, result is long
	if v.Type == TypeLong || other.Type == TypeLong {
		a, _ := v.toInt()
		b, _ := other.toInt()
		return LongValue(int64(a) + int64(b))
	}
	// Integer arithmetic with 32-bit wraparound
	if v.Type == TypeInteger && other.Type == TypeInteger {
		return IntegerValue(wrapInt32(v.Data.(int) + other.Data.(int)))
	}
	return NullValue()
}

func safeVal(v *Value) *Value {
	if v == nil {
		return NullValue()
	}
	return v
}

// Subtract returns v - other.
func (v *Value) Subtract(other *Value) *Value {
	panicNullPointer(v, other)
	if v.Type == TypeDouble || other.Type == TypeDouble {
		a, _ := v.toFloat64()
		b, _ := other.toFloat64()
		return DoubleValue(a - b)
	}
	if v.Type == TypeLong || other.Type == TypeLong {
		a, _ := v.toInt()
		b, _ := other.toInt()
		return LongValue(int64(a) - int64(b))
	}
	if v.Type == TypeInteger && other.Type == TypeInteger {
		return IntegerValue(wrapInt32(v.Data.(int) - other.Data.(int)))
	}
	return NullValue()
}

// Multiply returns v * other.
func (v *Value) Multiply(other *Value) *Value {
	panicNullPointer(v, other)
	if v.Type == TypeDouble || other.Type == TypeDouble {
		a, _ := v.toFloat64()
		b, _ := other.toFloat64()
		return DoubleValue(a * b)
	}
	if v.Type == TypeLong || other.Type == TypeLong {
		a, _ := v.toInt()
		b, _ := other.toInt()
		return LongValue(int64(a) * int64(b))
	}
	if v.Type == TypeInteger && other.Type == TypeInteger {
		return IntegerValue(wrapInt32(v.Data.(int) * other.Data.(int)))
	}
	return NullValue()
}

// Divide returns v / other.
func (v *Value) Divide(other *Value) *Value {
	panicNullPointer(v, other)
	if v.Type == TypeDouble || other.Type == TypeDouble {
		a, _ := v.toFloat64()
		b, _ := other.toFloat64()
		if b == 0 {
			panic(&ArithmeticError{Message: "Divide by 0"})
		}
		return DoubleValue(a / b)
	}
	if v.Type == TypeLong || other.Type == TypeLong {
		a, _ := v.toInt()
		b, _ := other.toInt()
		if b == 0 {
			panic(&ArithmeticError{Message: "Divide by 0"})
		}
		return LongValue(int64(a) / int64(b))
	}
	if v.Type == TypeInteger && other.Type == TypeInteger {
		b := other.Data.(int)
		if b == 0 {
			panic(&ArithmeticError{Message: "Divide by 0"})
		}
		return IntegerValue(wrapInt32(v.Data.(int) / b))
	}
	a, _ := v.toFloat64()
	b, _ := other.toFloat64()
	if b == 0 {
		panic(&ArithmeticError{Message: "Divide by 0"})
	}
	return DoubleValue(a / b)
}

// Modulo returns v % other.
func (v *Value) Modulo(other *Value) *Value {
	panicNullPointer(v, other)
	if v.Type == TypeInteger && other.Type == TypeInteger {
		b := other.Data.(int)
		if b == 0 {
			panic(&ArithmeticError{Message: "Divide by 0"})
		}
		return IntegerValue(wrapInt32(v.Data.(int) % b))
	}
	if v.isNumeric() && other.isNumeric() {
		a, _ := v.toFloat64()
		b, _ := other.toFloat64()
		if b == 0 {
			panic(&ArithmeticError{Message: "Divide by 0"})
		}
		return DoubleValue(math.Mod(a, b))
	}
	return NullValue()
}

// Equals returns true if two values are equal.
func (v *Value) Equals(other *Value) bool {
	if v == nil && other == nil {
		return true
	}
	if v == nil || other == nil {
		return (v == nil || v.Type == TypeNull) && (other == nil || other.Type == TypeNull)
	}
	if v.Type == TypeNull && other.Type == TypeNull {
		return true
	}
	if v.Type == TypeNull || other.Type == TypeNull {
		return false
	}
	// Numeric comparison with promotion
	if v.isNumeric() && other.isNumeric() {
		a, _ := v.toFloat64()
		b, _ := other.toFloat64()
		return a == b
	}
	if v.Type != other.Type {
		return false
	}
	switch v.Type {
	case TypeBoolean:
		return v.Data.(bool) == other.Data.(bool)
	case TypeString:
		return strings.EqualFold(v.Data.(string), other.Data.(string))
	}
	return v.ToString() == other.ToString()
}

// LessThan returns true if v < other.
func (v *Value) LessThan(other *Value) bool {
	if v == nil || other == nil {
		return false
	}
	if v.isNumeric() && other.isNumeric() {
		a, _ := v.toFloat64()
		b, _ := other.toFloat64()
		return a < b
	}
	if v.Type == TypeString && other.Type == TypeString {
		return strings.ToLower(v.Data.(string)) < strings.ToLower(other.Data.(string))
	}
	return false
}

// GreaterThan returns true if v > other.
func (v *Value) GreaterThan(other *Value) bool {
	if v == nil || other == nil {
		return false
	}
	if v.isNumeric() && other.isNumeric() {
		a, _ := v.toFloat64()
		b, _ := other.toFloat64()
		return a > b
	}
	if v.Type == TypeString && other.Type == TypeString {
		return strings.ToLower(v.Data.(string)) > strings.ToLower(other.Data.(string))
	}
	return false
}
