package interpreter

import (
	"crypto/hmac"
	"crypto/md5"
	crand "crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"math/rand"
	"strings"
)

// callCryptoMethod handles Crypto.* static method calls.
func callCryptoMethod(method string, args []*Value) (*Value, bool) {
	switch method {
	case "generatedigest":
		if len(args) >= 2 {
			algorithm := args[0].ToString()
			data := args[1].ToString()
			digest := computeDigest(algorithm, []byte(data))
			return StringValue(hex.EncodeToString(digest)), true
		}
		return NullValue(), true
	case "generatemac":
		if len(args) >= 3 {
			algorithm := args[0].ToString()
			data := args[1].ToString()
			key := args[2].ToString()
			mac := computeHMAC(algorithm, []byte(data), []byte(key))
			return StringValue(hex.EncodeToString(mac)), true
		}
		return NullValue(), true
	case "generateaeskey":
		if len(args) >= 1 {
			bits, _ := args[0].toInt()
			key := make([]byte, bits/8)
			_, _ = crand.Read(key)
			return StringValue(hex.EncodeToString(key)), true
		}
		return NullValue(), true
	case "getrandominteger":
		return IntegerValue(rand.Int()), true
	case "getrandomlong":
		return LongValue(rand.Int63()), true
	case "encrypt", "encryptwithmanagediv":
		// Simplified: return base64 of input for testing purposes
		if len(args) >= 3 {
			return StringValue(base64.StdEncoding.EncodeToString([]byte(args[2].ToString()))), true
		}
		return NullValue(), true
	case "decrypt", "decryptwithmanagediv":
		if len(args) >= 3 {
			decoded, _ := base64.StdEncoding.DecodeString(args[2].ToString())
			return StringValue(string(decoded)), true
		}
		return NullValue(), true
	}
	return nil, false
}

func computeDigest(algorithm string, data []byte) []byte {
	var h hash.Hash
	switch strings.ToUpper(algorithm) {
	case "MD5":
		h = md5.New()
	case "SHA1", "SHA-1":
		h = sha1.New()
	case "SHA256", "SHA-256":
		h = sha256.New()
	case "SHA512", "SHA-512":
		h = sha512.New()
	default:
		h = sha256.New()
	}
	h.Write(data)
	return h.Sum(nil)
}

func computeHMAC(algorithm string, data, key []byte) []byte {
	var newHash func() hash.Hash
	switch strings.ToUpper(algorithm) {
	case "HMACMD5":
		newHash = md5.New
	case "HMACSHA1":
		newHash = sha1.New
	case "HMACSHA256":
		newHash = sha256.New
	case "HMACSHA512":
		newHash = sha512.New
	default:
		newHash = sha256.New
	}
	mac := hmac.New(newHash, key)
	mac.Write(data)
	return mac.Sum(nil)
}
