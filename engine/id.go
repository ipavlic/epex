package engine

import (
	"fmt"
	"strings"
	"sync"
)

// knownPrefixes maps SObject names (lowercased) to their standard
// Salesforce 3-character key prefixes.
var knownPrefixes = map[string]string{
	"account":     "001",
	"contact":     "003",
	"opportunity": "006",
	"lead":        "00Q",
	"case":        "500",
	"user":        "005",
	"task":        "00T",
	"event":       "00U",
	"product2":    "01t",
	"pricebook2":  "01s",
	"order":       "801",
	"campaign":    "701",
	"contract":    "800",
	"asset":       "02i",
	"solution":    "501",
}

// IDGenerator generates Salesforce-style 18-character IDs.
type IDGenerator struct {
	mu       sync.Mutex
	counters map[string]int64
}

// NewIDGenerator creates a new IDGenerator.
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{
		counters: make(map[string]int64),
	}
}

// Generate produces a Salesforce-style 18-character ID for the given SObject.
func (g *IDGenerator) Generate(sobjectName string) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	key := strings.ToLower(sobjectName)
	g.counters[key]++
	counter := g.counters[key]

	prefix := knownPrefixes[key]
	if prefix == "" {
		// Default: "a0" + first character of the SObject name (uppercased).
		if len(sobjectName) > 0 {
			prefix = "a0" + strings.ToUpper(sobjectName[:1])
		} else {
			prefix = "a00"
		}
	}

	// Build 18-char ID: 3-char prefix + 12-digit zero-padded counter + 3-char suffix "AAA"
	return fmt.Sprintf("%s%012dAAA", prefix, counter)
}

// Reset clears all counters so IDs start from 1 again.
func (g *IDGenerator) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counters = make(map[string]int64)
}

// reversePrefixes maps 3-character key prefixes back to lowercased SObject names.
var reversePrefixes map[string]string

func init() {
	reversePrefixes = make(map[string]string, len(knownPrefixes))
	for name, prefix := range knownPrefixes {
		reversePrefixes[prefix] = name // name is already lowercase
	}
}

// SObjectTypeForID returns the SObject type name for a given Salesforce-style ID
// by examining its 3-character prefix. It checks standard prefixes first, then
// custom-object prefixes by scanning the generator's counters.
func (g *IDGenerator) SObjectTypeForID(id string) string {
	if len(id) < 3 {
		return ""
	}
	prefix := id[:3]

	// Check standard prefixes.
	if name, ok := reversePrefixes[prefix]; ok {
		return name
	}

	// Check custom-object prefixes ("a0" + first char) by scanning counters.
	if len(prefix) == 3 && prefix[:2] == "a0" {
		g.mu.Lock()
		defer g.mu.Unlock()
		for key := range g.counters {
			if _, ok := knownPrefixes[key]; ok {
				continue // standard object, already checked
			}
			// Custom objects get prefix "a0" + uppercase first char
			if len(key) > 0 && "a0"+strings.ToUpper(key[:1]) == prefix {
				return key
			}
		}
	}

	return ""
}
