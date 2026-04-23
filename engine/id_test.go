package engine

import "testing"

func TestSObjectTypeForID_StandardObjects(t *testing.T) {
	g := NewIDGenerator()

	tests := []struct {
		sobject  string
		expected string // reverse lookup returns lowercase
		prefix   string
	}{
		{"Account", "account", "001"},
		{"Contact", "contact", "003"},
		{"Opportunity", "opportunity", "006"},
		{"Lead", "lead", "00Q"},
		{"Task", "task", "00T"},
		{"User", "user", "005"},
	}

	for _, tt := range tests {
		// Generate an ID so the counter is populated.
		id := g.Generate(tt.sobject)

		got := g.SObjectTypeForID(id)
		if got != tt.expected {
			t.Errorf("SObjectTypeForID(%q) = %q, want %q (prefix %s)", id, got, tt.expected, tt.prefix)
		}
	}
}

func TestSObjectTypeForID_CustomObject(t *testing.T) {
	g := NewIDGenerator()

	// Generate IDs for a custom object to register it in counters.
	id := g.Generate("MyCustom__c")

	got := g.SObjectTypeForID(id)
	if got != "mycustom__c" {
		t.Errorf("SObjectTypeForID(%q) = %q, want %q", id, got, "mycustom__c")
	}
}

func TestSObjectTypeForID_UnknownPrefix(t *testing.T) {
	g := NewIDGenerator()

	got := g.SObjectTypeForID("ZZZ000000000001AAA")
	if got != "" {
		t.Errorf("SObjectTypeForID with unknown prefix = %q, want empty", got)
	}
}

func TestSObjectTypeForID_ShortID(t *testing.T) {
	g := NewIDGenerator()

	got := g.SObjectTypeForID("ab")
	if got != "" {
		t.Errorf("SObjectTypeForID with short ID = %q, want empty", got)
	}
}
