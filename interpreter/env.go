package interpreter

import "strings"

// Environment represents a variable scope.
type Environment struct {
	parent    *Environment
	variables map[string]*Value
}

// NewEnvironment creates a new environment with an optional parent scope.
func NewEnvironment(parent *Environment) *Environment {
	return &Environment{
		parent:    parent,
		variables: make(map[string]*Value),
	}
}

// Get looks up a variable by name, walking up the scope chain.
// Variable lookup is case-insensitive for Apex compatibility.
func (e *Environment) Get(name string) (*Value, bool) {
	lower := strings.ToLower(name)
	for k, v := range e.variables {
		if strings.ToLower(k) == lower {
			return v, true
		}
	}
	if e.parent != nil {
		return e.parent.Get(name)
	}
	return nil, false
}

// Set sets a variable in the scope where it is defined. If not found, sets in the current scope.
func (e *Environment) Set(name string, value *Value) {
	lower := strings.ToLower(name)
	// Check if variable exists in this scope
	for k := range e.variables {
		if strings.ToLower(k) == lower {
			e.variables[k] = value
			return
		}
	}
	// Check parent scopes
	if e.parent != nil {
		if _, ok := e.parent.Get(name); ok {
			e.parent.Set(name, value)
			return
		}
	}
	// Define in current scope
	e.variables[name] = value
}

// Define defines a variable in the current scope.
func (e *Environment) Define(name string, value *Value) {
	e.variables[name] = value
}
