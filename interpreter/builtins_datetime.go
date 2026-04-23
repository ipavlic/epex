package interpreter

import (
	"fmt"
	"strings"
	"time"
)

// DateValue creates a Date value (stored as SObject with SType "Date").
func DateValue(t time.Time) *Value {
	y, m, d := t.Date()
	fields := map[string]*Value{
		"year":  IntegerValue(y),
		"month": IntegerValue(int(m)),
		"day":   IntegerValue(d),
		"_time": &Value{Type: TypeLong, Data: t.Unix()},
	}
	return &Value{Type: TypeSObject, Data: fields, SType: "Date"}
}

// DatetimeValue creates a Datetime value.
func DatetimeValue(t time.Time) *Value {
	fields := map[string]*Value{
		"year":        IntegerValue(t.Year()),
		"month":       IntegerValue(int(t.Month())),
		"day":         IntegerValue(t.Day()),
		"hour":        IntegerValue(t.Hour()),
		"minute":      IntegerValue(t.Minute()),
		"second":      IntegerValue(t.Second()),
		"millisecond": IntegerValue(t.Nanosecond() / 1e6),
		"_time":       &Value{Type: TypeLong, Data: t.Unix()},
	}
	return &Value{Type: TypeSObject, Data: fields, SType: "Datetime"}
}

func dateFromValue(v *Value) time.Time {
	if v.Type != TypeSObject {
		return time.Time{}
	}
	fields := v.Data.(map[string]*Value)
	if tv, ok := fields["_time"]; ok && tv.Type == TypeLong {
		return time.Unix(tv.Data.(int64), 0).UTC()
	}
	y, _ := getFieldInt(fields, "year")
	m, _ := getFieldInt(fields, "month")
	d, _ := getFieldInt(fields, "day")
	h, _ := getFieldInt(fields, "hour")
	min, _ := getFieldInt(fields, "minute")
	sec, _ := getFieldInt(fields, "second")
	return time.Date(y, time.Month(m), d, h, min, sec, 0, time.UTC)
}

func getFieldInt(fields map[string]*Value, key string) (int, bool) {
	if v, ok := fields[key]; ok {
		return v.toInt()
	}
	return 0, false
}

// callDateStaticMethod handles Date.* static methods.
func callDateStaticMethod(method string, args []*Value) (*Value, bool) {
	switch method {
	case "today":
		return DateValue(time.Now().UTC()), true
	case "newinstance":
		if len(args) >= 3 {
			y, _ := args[0].toInt()
			m, _ := args[1].toInt()
			d, _ := args[2].toInt()
			return DateValue(time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)), true
		}
		return NullValue(), true
	case "parse":
		if len(args) >= 1 {
			s := args[0].ToString()
			for _, layout := range []string{"2006-01-02", "01/02/2006", "1/2/2006"} {
				if t, err := time.Parse(layout, s); err == nil {
					return DateValue(t), true
				}
			}
		}
		return NullValue(), true
	case "valueof":
		if len(args) >= 1 {
			s := args[0].ToString()
			if t, err := time.Parse("2006-01-02", s); err == nil {
				return DateValue(t), true
			}
		}
		return NullValue(), true
	case "isleapyear":
		if len(args) >= 1 {
			y, _ := args[0].toInt()
			return BooleanValue(y%4 == 0 && (y%100 != 0 || y%400 == 0)), true
		}
		return BooleanValue(false), true
	case "daysinmonth":
		if len(args) >= 2 {
			y, _ := args[0].toInt()
			m, _ := args[1].toInt()
			t := time.Date(y, time.Month(m+1), 0, 0, 0, 0, 0, time.UTC)
			return IntegerValue(t.Day()), true
		}
		return IntegerValue(0), true
	}
	return nil, false
}

// callDatetimeStaticMethod handles Datetime.* static methods.
func callDatetimeStaticMethod(method string, args []*Value) (*Value, bool) {
	switch method {
	case "now":
		return DatetimeValue(time.Now().UTC()), true
	case "newinstance":
		if len(args) >= 6 {
			y, _ := args[0].toInt()
			mo, _ := args[1].toInt()
			d, _ := args[2].toInt()
			h, _ := args[3].toInt()
			mi, _ := args[4].toInt()
			s, _ := args[5].toInt()
			return DatetimeValue(time.Date(y, time.Month(mo), d, h, mi, s, 0, time.UTC)), true
		}
		// newInstance(Long) - epoch millis
		if len(args) >= 1 {
			f, _ := args[0].toFloat64()
			return DatetimeValue(time.Unix(int64(f)/1000, (int64(f)%1000)*1e6).UTC()), true
		}
		return NullValue(), true
	case "newinstancegmt":
		if len(args) >= 6 {
			y, _ := args[0].toInt()
			mo, _ := args[1].toInt()
			d, _ := args[2].toInt()
			h, _ := args[3].toInt()
			mi, _ := args[4].toInt()
			s, _ := args[5].toInt()
			return DatetimeValue(time.Date(y, time.Month(mo), d, h, mi, s, 0, time.UTC)), true
		}
		return NullValue(), true
	case "parse":
		if len(args) >= 1 {
			s := args[0].ToString()
			for _, layout := range []string{
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05Z",
				"2006-01-02T15:04:05.000Z",
				time.RFC3339,
			} {
				if t, err := time.Parse(layout, s); err == nil {
					return DatetimeValue(t), true
				}
			}
		}
		return NullValue(), true
	case "valueof":
		if len(args) >= 1 {
			s := args[0].ToString()
			for _, layout := range []string{
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05Z",
				time.RFC3339,
			} {
				if t, err := time.Parse(layout, s); err == nil {
					return DatetimeValue(t), true
				}
			}
		}
		return NullValue(), true
	case "valueofgmt":
		if len(args) >= 1 {
			s := args[0].ToString()
			if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
				return DatetimeValue(t), true
			}
		}
		return NullValue(), true
	}
	return nil, false
}

// callDatetimeInstanceMethod handles instance methods on Date and Datetime values.
func callDatetimeInstanceMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	t := dateFromValue(obj)
	fields := obj.Data.(map[string]*Value)

	switch method {
	case "year":
		return IntegerValue(t.Year()), true
	case "month":
		return IntegerValue(int(t.Month())), true
	case "day":
		return IntegerValue(t.Day()), true
	case "dayofyear":
		return IntegerValue(t.YearDay()), true
	case "hour":
		return IntegerValue(t.Hour()), true
	case "minute":
		return IntegerValue(t.Minute()), true
	case "second":
		return IntegerValue(t.Second()), true
	case "millisecond":
		if v, ok := fields["millisecond"]; ok {
			i, _ := v.toInt()
			return IntegerValue(i), true
		}
		return IntegerValue(0), true
	case "gettime":
		return LongValue(t.UnixMilli()), true
	case "date":
		return DateValue(t), true
	case "adddays":
		if len(args) >= 1 {
			d, _ := args[0].toInt()
			newT := t.AddDate(0, 0, d)
			if obj.SType == "Date" {
				return DateValue(newT), true
			}
			return DatetimeValue(newT), true
		}
		return obj, true
	case "addmonths":
		if len(args) >= 1 {
			m, _ := args[0].toInt()
			newT := t.AddDate(0, m, 0)
			if obj.SType == "Date" {
				return DateValue(newT), true
			}
			return DatetimeValue(newT), true
		}
		return obj, true
	case "addyears":
		if len(args) >= 1 {
			y, _ := args[0].toInt()
			newT := t.AddDate(y, 0, 0)
			if obj.SType == "Date" {
				return DateValue(newT), true
			}
			return DatetimeValue(newT), true
		}
		return obj, true
	case "addhours":
		if len(args) >= 1 {
			h, _ := args[0].toInt()
			return DatetimeValue(t.Add(time.Duration(h) * time.Hour)), true
		}
		return obj, true
	case "addminutes":
		if len(args) >= 1 {
			m, _ := args[0].toInt()
			return DatetimeValue(t.Add(time.Duration(m) * time.Minute)), true
		}
		return obj, true
	case "addseconds":
		if len(args) >= 1 {
			s, _ := args[0].toInt()
			return DatetimeValue(t.Add(time.Duration(s) * time.Second)), true
		}
		return obj, true
	case "daysbetween":
		if len(args) >= 1 {
			other := dateFromValue(args[0])
			diff := other.Sub(t)
			return IntegerValue(int(diff.Hours() / 24)), true
		}
		return IntegerValue(0), true
	case "issameday":
		if len(args) >= 1 {
			other := dateFromValue(args[0])
			return BooleanValue(t.Year() == other.Year() && t.YearDay() == other.YearDay()), true
		}
		return BooleanValue(false), true
	case "format":
		if obj.SType == "Date" {
			return StringValue(t.Format("2006-01-02")), true
		}
		return StringValue(t.Format("2006-01-02 15:04:05")), true
	case "formatgmt":
		if len(args) >= 1 {
			pattern := args[0].ToString()
			return StringValue(apexDateFormatToGo(t, pattern)), true
		}
		return StringValue(t.Format("2006-01-02T15:04:05.000Z")), true
	case "tostring":
		if obj.SType == "Date" {
			return StringValue(t.Format("2006-01-02")), true
		}
		return StringValue(t.Format("2006-01-02 15:04:05")), true
	case "isleapyear":
		y := t.Year()
		return BooleanValue(y%4 == 0 && (y%100 != 0 || y%400 == 0)), true
	case "numberofdays":
		// Days in the month
		next := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC)
		return IntegerValue(next.Day()), true
	}
	return nil, false
}

// apexDateFormatToGo converts simple Apex/Java date format patterns to Go output.
func apexDateFormatToGo(t time.Time, pattern string) string {
	// Simple conversion of common patterns
	r := pattern
	r = strings.ReplaceAll(r, "yyyy", fmt.Sprintf("%04d", t.Year()))
	r = strings.ReplaceAll(r, "yy", fmt.Sprintf("%02d", t.Year()%100))
	r = strings.ReplaceAll(r, "MM", fmt.Sprintf("%02d", int(t.Month())))
	r = strings.ReplaceAll(r, "dd", fmt.Sprintf("%02d", t.Day()))
	r = strings.ReplaceAll(r, "HH", fmt.Sprintf("%02d", t.Hour()))
	r = strings.ReplaceAll(r, "mm", fmt.Sprintf("%02d", t.Minute()))
	r = strings.ReplaceAll(r, "ss", fmt.Sprintf("%02d", t.Second()))
	return r
}