package interpreter

import (
	"fmt"
	"strings"
	"unicode"
)

// extendedStringMethod handles String instance methods beyond the basic set in builtins.go.
// Returns (result, true) if handled, (nil, false) if not.
func extendedStringMethod(s string, method string, args []*Value) (*Value, bool) {
	switch method {
	case "charat":
		if len(args) >= 1 {
			idx, _ := args[0].toInt()
			if idx >= 0 && idx < len(s) {
				return IntegerValue(int(s[idx])), true
			}
		}
		return IntegerValue(0), true
	case "codepointat":
		if len(args) >= 1 {
			idx, _ := args[0].toInt()
			runes := []rune(s)
			if idx >= 0 && idx < len(runes) {
				return IntegerValue(int(runes[idx])), true
			}
		}
		return IntegerValue(0), true
	case "compareto":
		if len(args) >= 1 {
			other := args[0].ToString()
			return IntegerValue(strings.Compare(s, other)), true
		}
		return IntegerValue(0), true
	case "comparetoignorecase":
		if len(args) >= 1 {
			other := args[0].ToString()
			return IntegerValue(strings.Compare(strings.ToLower(s), strings.ToLower(other))), true
		}
		return IntegerValue(0), true
	case "containsignorecase":
		if len(args) >= 1 {
			return BooleanValue(strings.Contains(strings.ToLower(s), strings.ToLower(args[0].ToString()))), true
		}
		return BooleanValue(false), true
	case "containsany":
		if len(args) >= 1 {
			chars := args[0].ToString()
			return BooleanValue(strings.ContainsAny(s, chars)), true
		}
		return BooleanValue(false), true
	case "containsnone":
		if len(args) >= 1 {
			chars := args[0].ToString()
			return BooleanValue(!strings.ContainsAny(s, chars)), true
		}
		return BooleanValue(true), true
	case "containsonly":
		if len(args) >= 1 {
			allowed := args[0].ToString()
			for _, c := range s {
				if !strings.ContainsRune(allowed, c) {
					return BooleanValue(false), true
				}
			}
			return BooleanValue(true), true
		}
		return BooleanValue(true), true
	case "containswhitespace":
		for _, c := range s {
			if unicode.IsSpace(c) {
				return BooleanValue(true), true
			}
		}
		return BooleanValue(false), true
	case "countmatches":
		if len(args) >= 1 {
			return IntegerValue(strings.Count(s, args[0].ToString())), true
		}
		return IntegerValue(0), true
	case "deletewhitespace":
		var b strings.Builder
		for _, c := range s {
			if !unicode.IsSpace(c) {
				b.WriteRune(c)
			}
		}
		return StringValue(b.String()), true
	case "endswithignorecase":
		if len(args) >= 1 {
			return BooleanValue(strings.HasSuffix(strings.ToLower(s), strings.ToLower(args[0].ToString()))), true
		}
		return BooleanValue(false), true
	case "startswithignorecase":
		if len(args) >= 1 {
			return BooleanValue(strings.HasPrefix(strings.ToLower(s), strings.ToLower(args[0].ToString()))), true
		}
		return BooleanValue(false), true
	case "escapesinglequotes":
		return StringValue(strings.ReplaceAll(s, "'", "\\'")), true
	case "hashcode":
		h := 0
		for _, c := range s {
			h = 31*h + int(c)
		}
		return IntegerValue(wrapInt32(h)), true
	case "indexofignorecase":
		if len(args) >= 1 {
			return IntegerValue(strings.Index(strings.ToLower(s), strings.ToLower(args[0].ToString()))), true
		}
		return IntegerValue(-1), true
	case "isalluppercase":
		return BooleanValue(s == strings.ToUpper(s) && len(s) > 0), true
	case "isalllowercase":
		return BooleanValue(s == strings.ToLower(s) && len(s) > 0), true
	case "isalpha":
		if len(s) == 0 {
			return BooleanValue(false), true
		}
		for _, c := range s {
			if !unicode.IsLetter(c) {
				return BooleanValue(false), true
			}
		}
		return BooleanValue(true), true
	case "isalphanumeric":
		if len(s) == 0 {
			return BooleanValue(false), true
		}
		for _, c := range s {
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
				return BooleanValue(false), true
			}
		}
		return BooleanValue(true), true
	case "isalphanumericspace":
		if len(s) == 0 {
			return BooleanValue(false), true
		}
		for _, c := range s {
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != ' ' {
				return BooleanValue(false), true
			}
		}
		return BooleanValue(true), true
	case "isalphaspace":
		if len(s) == 0 {
			return BooleanValue(false), true
		}
		for _, c := range s {
			if !unicode.IsLetter(c) && c != ' ' {
				return BooleanValue(false), true
			}
		}
		return BooleanValue(true), true
	case "isblank":
		return BooleanValue(strings.TrimSpace(s) == ""), true
	case "isnotblank":
		return BooleanValue(strings.TrimSpace(s) != ""), true
	case "isempty":
		return BooleanValue(len(s) == 0), true
	case "isnotempty":
		return BooleanValue(len(s) > 0), true
	case "isnumeric":
		if len(s) == 0 {
			return BooleanValue(false), true
		}
		for _, c := range s {
			if !unicode.IsDigit(c) {
				return BooleanValue(false), true
			}
		}
		return BooleanValue(true), true
	case "iswhitespace":
		if len(s) == 0 {
			return BooleanValue(false), true
		}
		for _, c := range s {
			if !unicode.IsSpace(c) {
				return BooleanValue(false), true
			}
		}
		return BooleanValue(true), true
	case "lastindexof":
		if len(args) >= 1 {
			return IntegerValue(strings.LastIndex(s, args[0].ToString())), true
		}
		return IntegerValue(-1), true
	case "lastindexofignorecase":
		if len(args) >= 1 {
			return IntegerValue(strings.LastIndex(strings.ToLower(s), strings.ToLower(args[0].ToString()))), true
		}
		return IntegerValue(-1), true
	case "left":
		if len(args) >= 1 {
			n, _ := args[0].toInt()
			if n <= 0 {
				return StringValue(""), true
			}
			if n > len(s) {
				n = len(s)
			}
			return StringValue(s[:n]), true
		}
		return StringValue(""), true
	case "leftpad":
		if len(args) >= 1 {
			n, _ := args[0].toInt()
			pad := " "
			if len(args) >= 2 {
				pad = args[1].ToString()
			}
			for len(s) < n && len(pad) > 0 {
				s = pad + s
			}
			if len(s) > n {
				s = s[len(s)-n:]
			}
			return StringValue(s), true
		}
		return StringValue(s), true
	case "right":
		if len(args) >= 1 {
			n, _ := args[0].toInt()
			if n <= 0 {
				return StringValue(""), true
			}
			if n > len(s) {
				n = len(s)
			}
			return StringValue(s[len(s)-n:]), true
		}
		return StringValue(""), true
	case "rightpad":
		if len(args) >= 1 {
			n, _ := args[0].toInt()
			pad := " "
			if len(args) >= 2 {
				pad = args[1].ToString()
			}
			for len(s) < n && len(pad) > 0 {
				s = s + pad
			}
			if len(s) > n {
				s = s[:n]
			}
			return StringValue(s), true
		}
		return StringValue(s), true
	case "mid":
		if len(args) >= 2 {
			start, _ := args[0].toInt()
			length, _ := args[1].toInt()
			if start < 0 {
				start = 0
			}
			if start >= len(s) {
				return StringValue(""), true
			}
			end := start + length
			if end > len(s) {
				end = len(s)
			}
			return StringValue(s[start:end]), true
		}
		return StringValue(""), true
	case "normalizeSpace":
		// not standard but commonly used
		return StringValue(strings.Join(strings.Fields(s), " ")), true
	case "remove":
		if len(args) >= 1 {
			return StringValue(strings.ReplaceAll(s, args[0].ToString(), "")), true
		}
		return StringValue(s), true
	case "removeend":
		if len(args) >= 1 {
			suffix := args[0].ToString()
			return StringValue(strings.TrimSuffix(s, suffix)), true
		}
		return StringValue(s), true
	case "removeendignorecase":
		if len(args) >= 1 {
			suffix := args[0].ToString()
			if strings.HasSuffix(strings.ToLower(s), strings.ToLower(suffix)) {
				return StringValue(s[:len(s)-len(suffix)]), true
			}
		}
		return StringValue(s), true
	case "removestart":
		if len(args) >= 1 {
			prefix := args[0].ToString()
			return StringValue(strings.TrimPrefix(s, prefix)), true
		}
		return StringValue(s), true
	case "removestartignorecase":
		if len(args) >= 1 {
			prefix := args[0].ToString()
			if strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix)) {
				return StringValue(s[len(prefix):]), true
			}
		}
		return StringValue(s), true
	case "repeat":
		if len(args) >= 1 {
			n, _ := args[0].toInt()
			if n <= 0 {
				return StringValue(""), true
			}
			return StringValue(strings.Repeat(s, n)), true
		}
		return StringValue(s), true
	case "replaceall":
		if len(args) >= 2 {
			// In Apex, replaceAll uses regex
			return StringValue(strings.ReplaceAll(s, args[0].ToString(), args[1].ToString())), true
		}
		return StringValue(s), true
	case "replacefirst":
		if len(args) >= 2 {
			return StringValue(strings.Replace(s, args[0].ToString(), args[1].ToString(), 1)), true
		}
		return StringValue(s), true
	case "reverse":
		runes := []rune(s)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return StringValue(string(runes)), true
	case "splitbychartypecamelcase":
		// Simple camelCase split
		var parts []string
		start := 0
		for i := 1; i < len(s); i++ {
			if unicode.IsUpper(rune(s[i])) {
				parts = append(parts, s[start:i])
				start = i
			}
		}
		parts = append(parts, s[start:])
		elements := make([]*Value, len(parts))
		for i, p := range parts {
			elements[i] = StringValue(p)
		}
		return ListValue(elements), true
	case "striphtml", "striptags", "striphmltags":
		// Simple HTML tag removal
		result := s
		for {
			open := strings.Index(result, "<")
			if open < 0 {
				break
			}
			close := strings.Index(result[open:], ">")
			if close < 0 {
				break
			}
			result = result[:open] + result[open+close+1:]
		}
		return StringValue(result), true
	case "tointeger":
		return StringValue(s), true // will be converted by caller
	case "tolabel":
		return StringValue(s), true
	case "tochararray":
		runes := []rune(s)
		elements := make([]*Value, len(runes))
		for i, r := range runes {
			elements[i] = StringValue(string(r))
		}
		return ListValue(elements), true
	case "uncapitalize":
		if len(s) == 0 {
			return StringValue(""), true
		}
		return StringValue(strings.ToLower(s[:1]) + s[1:]), true
	case "capitalize":
		if len(s) == 0 {
			return StringValue(""), true
		}
		return StringValue(strings.ToUpper(s[:1]) + s[1:]), true
	case "abbreviate":
		if len(args) >= 1 {
			maxLen, _ := args[0].toInt()
			if maxLen >= len(s) || maxLen < 4 {
				return StringValue(s), true
			}
			return StringValue(s[:maxLen-3] + "..."), true
		}
		return StringValue(s), true
	case "center":
		if len(args) >= 1 {
			size, _ := args[0].toInt()
			pad := " "
			if len(args) >= 2 {
				pad = args[1].ToString()
			}
			if size <= len(s) || len(pad) == 0 {
				return StringValue(s), true
			}
			totalPad := size - len(s)
			leftPad := totalPad / 2
			rightPad := totalPad - leftPad
			return StringValue(strings.Repeat(pad, leftPad)[:leftPad] + s + strings.Repeat(pad, rightPad)[:rightPad]), true
		}
		return StringValue(s), true
	case "format":
		// String.format uses Java MessageFormat-style {0}, {1}, etc.
		if len(args) >= 1 && args[0].Type == TypeList {
			result := s
			elements := args[0].Data.([]*Value)
			for i, e := range elements {
				placeholder := fmt.Sprintf("{%d}", i)
				result = strings.ReplaceAll(result, placeholder, e.ToString())
			}
			return StringValue(result), true
		}
		return StringValue(s), true
	case "normalizespace":
		return StringValue(strings.Join(strings.Fields(s), " ")), true
	case "swapcase":
		var b strings.Builder
		for _, c := range s {
			if unicode.IsUpper(c) {
				b.WriteRune(unicode.ToLower(c))
			} else if unicode.IsLower(c) {
				b.WriteRune(unicode.ToUpper(c))
			} else {
				b.WriteRune(c)
			}
		}
		return StringValue(b.String()), true
	case "matches":
		if len(args) >= 1 {
			// Simple match - in Apex this is regex
			return BooleanValue(s == args[0].ToString()), true
		}
		return BooleanValue(false), true
	case "codepoints":
		runes := []rune(s)
		elements := make([]*Value, len(runes))
		for i, r := range runes {
			elements[i] = IntegerValue(int(r))
		}
		return ListValue(elements), true
	case "getlevenshtein", "getlevensteindistance":
		if len(args) >= 1 {
			other := args[0].ToString()
			return IntegerValue(levenshtein(s, other)), true
		}
		return IntegerValue(0), true
	}
	return nil, false
}

// Static String methods beyond valueOf
func staticStringMethod(method string, args []*Value) (*Value, bool) {
	switch method {
	case "isblank":
		if len(args) >= 1 {
			if args[0].Type == TypeNull {
				return BooleanValue(true), true
			}
			return BooleanValue(strings.TrimSpace(args[0].ToString()) == ""), true
		}
		return BooleanValue(true), true
	case "isnotblank":
		if len(args) >= 1 {
			if args[0].Type == TypeNull {
				return BooleanValue(false), true
			}
			return BooleanValue(strings.TrimSpace(args[0].ToString()) != ""), true
		}
		return BooleanValue(false), true
	case "isempty":
		if len(args) >= 1 {
			if args[0].Type == TypeNull {
				return BooleanValue(true), true
			}
			return BooleanValue(args[0].ToString() == ""), true
		}
		return BooleanValue(true), true
	case "isnotempty":
		if len(args) >= 1 {
			if args[0].Type == TypeNull {
				return BooleanValue(false), true
			}
			return BooleanValue(args[0].ToString() != ""), true
		}
		return BooleanValue(false), true
	case "join":
		if len(args) >= 2 && args[0].Type == TypeList {
			elements := args[0].Data.([]*Value)
			sep := args[1].ToString()
			parts := make([]string, len(elements))
			for i, e := range elements {
				parts[i] = e.ToString()
			}
			return StringValue(strings.Join(parts, sep)), true
		}
		return StringValue(""), true
	case "escapesinglequotes":
		if len(args) >= 1 {
			return StringValue(strings.ReplaceAll(args[0].ToString(), "'", "\\'")), true
		}
		return StringValue(""), true
	case "format":
		if len(args) >= 1 {
			result := args[0].ToString()
			if len(args) >= 2 && args[1].Type == TypeList {
				elements := args[1].Data.([]*Value)
				for i, e := range elements {
					placeholder := fmt.Sprintf("{%d}", i)
					result = strings.ReplaceAll(result, placeholder, e.ToString())
				}
			}
			return StringValue(result), true
		}
		return StringValue(""), true
	case "fromchararray":
		if len(args) >= 1 && args[0].Type == TypeList {
			elements := args[0].Data.([]*Value)
			var b strings.Builder
			for _, e := range elements {
				b.WriteString(e.ToString())
			}
			return StringValue(b.String()), true
		}
		return StringValue(""), true
	}
	return nil, false
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
		}
	}
	return d[la][lb]
}

