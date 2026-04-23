package interpreter

import (
	"fmt"
	"strings"
	"time"
)

// TestResult holds the result of running a single test method.
type TestResult struct {
	ClassName  string
	MethodName string
	Passed     bool
	Error      string
	Duration   time.Duration
}

// RunTests discovers and runs all test methods across registered classes.
func (interp *Interpreter) RunTests() []*TestResult {
	var results []*TestResult

	for _, classInfo := range interp.registry.Classes {
		if !isTestClass(classInfo) {
			continue
		}
		for _, methodInfo := range classInfo.Methods {
			if !methodInfo.IsTest {
				continue
			}
			result := interp.runSingleTest(classInfo, methodInfo)
			results = append(results, result)
		}
	}
	return results
}

// RunTestsForClass runs all test methods in a specific class.
func (interp *Interpreter) RunTestsForClass(className string) []*TestResult {
	var results []*TestResult

	classInfo, ok := interp.registry.Classes[strings.ToLower(className)]
	if !ok {
		return results
	}

	for _, methodInfo := range classInfo.Methods {
		if !methodInfo.IsTest {
			continue
		}
		result := interp.runSingleTest(classInfo, methodInfo)
		results = append(results, result)
	}
	return results
}

func (interp *Interpreter) runSingleTest(classInfo *ClassInfo, methodInfo *MethodInfo) *TestResult {
	result := &TestResult{
		ClassName:  classInfo.Name,
		MethodName: methodInfo.Name,
	}

	start := time.Now()

	// Run in isolation with a fresh environment
	prevEnv := interp.env
	prevThis := interp.thisObj
	prevClass := interp.currentClass

	interp.env = NewEnvironment(nil)
	interp.currentClass = classInfo
	interp.currentFile = classInfo.SourceFile
	interp.thisObj = interp.createInstance(classInfo, nil)

	// Reset database state for test isolation — each test starts with empty tables.
	if interp.engine != nil {
		if err := interp.engine.ResetDatabase(); err != nil {
			result.Passed = false
			result.Error = fmt.Sprintf("failed to reset database: %v", err)
			result.Duration = time.Since(start)
			interp.env = prevEnv
			interp.thisObj = prevThis
			interp.currentClass = prevClass
			interp.currentFile = ""
			return result
		}
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				result.Passed = false
				if assertErr, ok := r.(*AssertException); ok {
					result.Error = assertErr.Error()
				} else {
					result.Error = fmt.Sprintf("%v", r)
				}
			}
		}()

		interp.executeMethodNode(methodInfo, nil)
		result.Passed = true
	}()

	result.Duration = time.Since(start)

	// Restore state
	interp.env = prevEnv
	interp.thisObj = prevThis
	interp.currentClass = prevClass
	interp.currentFile = ""

	return result
}

func isTestClass(classInfo *ClassInfo) bool {
	for _, ann := range classInfo.Annotations {
		if strings.EqualFold(ann, "isTest") {
			return true
		}
	}
	// Also check if any method has @isTest or testMethod
	for _, mi := range classInfo.Methods {
		if mi.IsTest {
			return true
		}
	}
	return false
}
