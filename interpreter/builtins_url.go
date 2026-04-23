package interpreter

import (
	"encoding/base64"
	"encoding/hex"
	"net/url"
)

// callUrlStaticMethod handles URL/EncodingUtil.* static method calls.
func callUrlStaticMethod(method string, args []*Value) (*Value, bool) {
	switch method {
	case "getfiledatefieldname":
		return NullValue(), true
	case "getorgdomainurl", "getsalesforcebaseurl":
		return StringValue("https://test.salesforce.com"), true
	case "getcurrentrequesturl":
		return StringValue("https://test.salesforce.com/apex/page"), true
	}
	return nil, false
}

// callEncodingUtilMethod handles EncodingUtil.* static method calls.
func callEncodingUtilMethod(method string, args []*Value) (*Value, bool) {
	switch method {
	case "urlencode":
		if len(args) >= 1 {
			return StringValue(url.QueryEscape(args[0].ToString())), true
		}
		return StringValue(""), true
	case "urldecode":
		if len(args) >= 1 {
			s := args[0].ToString()
			decoded, err := url.QueryUnescape(s)
			if err != nil {
				return StringValue(s), true
			}
			return StringValue(decoded), true
		}
		return StringValue(""), true
	case "base64encode":
		if len(args) >= 1 {
			// In Apex, this takes a Blob; we treat strings as blobs
			s := args[0].ToString()
			return StringValue(base64.StdEncoding.EncodeToString([]byte(s))), true
		}
		return StringValue(""), true
	case "base64decode":
		if len(args) >= 1 {
			s := args[0].ToString()
			decoded, _ := base64.StdEncoding.DecodeString(s)
			return StringValue(string(decoded)), true
		}
		return StringValue(""), true
	case "converttohex":
		if len(args) >= 1 {
			s := args[0].ToString()
			return StringValue(hex.EncodeToString([]byte(s))), true
		}
		return StringValue(""), true
	}
	return nil, false
}
