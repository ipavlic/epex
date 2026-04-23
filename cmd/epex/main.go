package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/ipavlic/epex/apex"
	"github.com/ipavlic/epex/engine"
	"github.com/ipavlic/epex/interpreter"
	"github.com/ipavlic/epex/reporter"
	"github.com/ipavlic/epex/schema"
	"github.com/ipavlic/epex/tracer"
)

var version = "dev"

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "epex",
		Short: "Local Apex interpreter and test runner",
		Long: `epex is a local Apex interpreter and test runner.

It parses Apex classes and triggers, builds an in-memory SObject schema
from object metadata, and executes @isTest methods with a local SQLite
database — no Salesforce org required.`,
		Version: version,
	}

	root.AddCommand(newTestCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newParseCmd())

	return root
}

// --- test command ---

func newTestCmd() *cobra.Command {
	var traceEnabled bool
	var traceFile string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "test <project-dir>",
		Short: "Run Apex tests",
		Long: `Parse Apex classes and triggers, build the SObject schema, and execute
all @isTest methods.

The project directory should follow the SFDX layout:

  project-dir/
    classes/        Apex .cls files
    triggers/       Apex .trigger files (optional)
    objects/        SObject metadata (.object-meta.xml)

Example:
  epex test ./force-app/main/default
  epex test --trace ./myproject`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTests(args[0], traceEnabled, traceFile, verbose)
		},
	}

	cmd.Flags().BoolVar(&traceEnabled, "trace", false, "Enable execution tracing and write a Perfetto trace file")
	cmd.Flags().StringVar(&traceFile, "trace-file", "trace.json", "Output path for the Perfetto trace JSON")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed parse and schema output")

	return cmd
}

func runTests(projectDir string, traceEnabled bool, traceFile string, verbose bool) error {
	commandStart := time.Now()

	// Step 1: Parse Apex classes and triggers
	classesDir := filepath.Join(projectDir, "classes")
	results, err := apex.ParseDirectory(classesDir)
	if err != nil {
		return fmt.Errorf("parsing classes: %w", err)
	}
	if verbose {
		for _, r := range results {
			status := "OK"
			if len(r.Errors) > 0 {
				status = fmt.Sprintf("ERRORS: %v", r.Errors)
			}
			fmt.Printf("  %-40s %s\n", r.Filename, status)
		}
	}

	triggerCount := 0
	triggersDir := filepath.Join(projectDir, "triggers")
	var trigResults []*apex.ParseResult
	if fi, statErr := os.Stat(triggersDir); statErr == nil && fi.IsDir() {
		trigResults, err = apex.ParseDirectory(triggersDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error parsing triggers: %v\n", err)
		}
	}

	// Step 2: Build schema
	objectsDir := filepath.Join(projectDir, "objects")
	s, err := schema.BuildSchemaFromDir(objectsDir)
	if err != nil {
		return fmt.Errorf("building schema: %w", err)
	}
	if verbose {
		for name, obj := range s.SObjects {
			fmt.Printf("  %s (%s)\n", name, obj.Label)
			for fieldName, field := range obj.Fields {
				extra := ""
				if field.ReferenceTo != "" {
					extra = fmt.Sprintf(" -> %s", field.ReferenceTo)
				}
				fmt.Printf("    %-30s %s%s\n", fieldName, field.Type, extra)
			}
		}
	}

	// Step 3: Create engine and register classes/triggers
	eng, err := engine.NewEngine(s)
	if err != nil {
		return fmt.Errorf("creating engine: %w", err)
	}
	defer eng.Close()

	reg := interpreter.NewRegistry()
	for _, r := range results {
		if r.Tree != nil {
			if !reg.RegisterTrigger(r.Tree, r.Filename) {
				reg.RegisterClass(r.Tree, r.Filename)
			}
		}
	}
	for _, r := range trigResults {
		if r.Tree != nil {
			if reg.RegisterTrigger(r.Tree, r.Filename) {
				triggerCount++
			}
		}
	}

	classCount := len(reg.Classes)
	fmt.Printf("Loaded %d class(es), %d trigger(s), %d SObject(s)\n\n",
		classCount, triggerCount, len(s.SObjects))

	interp := interpreter.NewInterpreter(reg, eng)

	// Enable tracing if requested
	var rec *tracer.RecordingTracer
	if traceEnabled {
		rec = tracer.NewRecordingTracer()
		interp.SetTracer(rec)
	}

	// Step 4: Run tests
	testResults := interp.RunTests()

	// Convert to reporter format and print results
	var reportResults []reporter.TestResult
	for _, tr := range testResults {
		outcome := reporter.OutcomePass
		msg := ""
		if !tr.Passed {
			outcome = reporter.OutcomeFail
			msg = tr.Error
		}
		reportResults = append(reportResults, reporter.TestResult{
			ClassName:  tr.ClassName,
			MethodName: tr.MethodName,
			Outcome:    outcome,
			Message:    msg,
			Duration:   tr.Duration,
		})
	}

	runResult := reporter.NewTestRunResult(reportResults, time.Since(commandStart))
	reporter.FormatHuman(os.Stdout, runResult)

	// Output trace if enabled
	if rec != nil {
		events := rec.Events()
		fmt.Println()

		summary := tracer.BuildSummary(events, 10)
		tracer.FormatSummaryHuman(os.Stdout, summary)

		f, err := os.Create(traceFile)
		if err != nil {
			return fmt.Errorf("creating trace file: %w", err)
		}
		defer f.Close()
		if err := tracer.WritePerfetto(f, events, rec.Epoch()); err != nil {
			return fmt.Errorf("writing trace: %w", err)
		}
		fmt.Printf("\nTrace written to %s (open at https://ui.perfetto.dev)\n", traceFile)
	}

	// Signal failure if any test failed
	for _, tr := range testResults {
		if !tr.Passed {
			return fmt.Errorf("test run failed: %d test(s) did not pass", runResult.Summary.Failing)
		}
	}

	return nil
}

// --- parse command ---

func newParseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parse <project-dir>",
		Short: "Parse and validate Apex source files",
		Long: `Parse all .cls and .trigger files under the project directory and
report any syntax errors. Does not execute tests.

Example:
  epex parse ./force-app/main/default`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runParse(args[0])
		},
	}
	return cmd
}

func runParse(projectDir string) error {
	dirs := []struct {
		path string
		kind string
	}{
		{filepath.Join(projectDir, "classes"), "class"},
		{filepath.Join(projectDir, "triggers"), "trigger"},
	}

	total, errCount := 0, 0
	for _, d := range dirs {
		if fi, err := os.Stat(d.path); err != nil || !fi.IsDir() {
			continue
		}
		results, err := apex.ParseDirectory(d.path)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", d.kind, err)
		}
		for _, r := range results {
			total++
			status := "OK"
			if len(r.Errors) > 0 {
				status = fmt.Sprintf("FAIL  %v", r.Errors)
				errCount++
			}
			fmt.Printf("  %-50s %s\n", r.Filename, status)
		}
	}

	fmt.Printf("\n%d file(s) parsed, %d error(s)\n", total, errCount)
	if errCount > 0 {
		return fmt.Errorf("%d file(s) had parse errors", errCount)
	}
	return nil
}

// --- run command ---

func newRunCmd() *cobra.Command {
	var projectDir string

	cmd := &cobra.Command{
		Use:   "run <file>",
		Short: "Execute an anonymous Apex file",
		Long: `Parse and execute an Apex file as anonymous code.

Optionally provide a project directory (--project) to load classes,
triggers, and SObject schema so the code can reference them.

Examples:
  epex run script.apex
  epex run --project ./force-app/main/default script.apex`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			return runAnonymous(string(code), projectDir)
		},
	}

	cmd.Flags().StringVarP(&projectDir, "project", "p", "", "Project directory to load classes and schema from")

	return cmd
}

func runAnonymous(code string, projectDir string) error {
	reg := interpreter.NewRegistry()
	var eng *engine.Engine

	if projectDir != "" {
		// Load classes
		classesDir := filepath.Join(projectDir, "classes")
		results, err := apex.ParseDirectory(classesDir)
		if err == nil {
			for _, r := range results {
				if r.Tree != nil {
					if !reg.RegisterTrigger(r.Tree, r.Filename) {
						reg.RegisterClass(r.Tree, r.Filename)
					}
				}
			}
		}

		// Load triggers
		triggersDir := filepath.Join(projectDir, "triggers")
		if fi, statErr := os.Stat(triggersDir); statErr == nil && fi.IsDir() {
			trigResults, err := apex.ParseDirectory(triggersDir)
			if err == nil {
				for _, r := range trigResults {
					if r.Tree != nil {
						reg.RegisterTrigger(r.Tree, r.Filename)
					}
				}
			}
		}

		// Load schema and create engine
		objectsDir := filepath.Join(projectDir, "objects")
		s, err := schema.BuildSchemaFromDir(objectsDir)
		if err != nil {
			s = schema.NewSchema()
		}
		eng, err = engine.NewEngine(s)
		if err != nil {
			return fmt.Errorf("creating engine: %w", err)
		}
		defer eng.Close()
	}

	// Wrap the code in a class so it can be parsed, then execute the method.
	wrapped := "public class AnonymousBlock { public static void execute() { " + code + " } }"
	result, err := apex.ParseString("anonymous", wrapped)
	if err != nil {
		return fmt.Errorf("parsing code: %w", err)
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("syntax errors: %v", result.Errors)
	}

	// Register the anonymous class so the interpreter knows about it.
	reg.RegisterClass(result.Tree, "anonymous")

	interp := interpreter.NewInterpreter(reg, eng)
	interp.ExecuteMethod("AnonymousBlock", "execute", nil)

	return nil
}
