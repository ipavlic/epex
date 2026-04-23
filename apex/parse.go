package apex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/ipavlic/epex/parser"
)

// ParseResult holds the result of parsing a single Apex file.
type ParseResult struct {
	Filename string
	Tree     parser.ICompilationUnitContext
	Stream   *antlr.CommonTokenStream
	Errors   []string
}

// ParseFile parses an Apex .cls file from disk.
func ParseFile(filename string) (*ParseResult, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filename, err)
	}
	return ParseString(filename, string(src))
}

// ParseString parses Apex source code from a string.
func ParseString(filename, source string) (*ParseResult, error) {
	input := antlr.NewInputStream(source)
	lexer := parser.NewApexLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	p := parser.NewApexParser(stream)
	p.RemoveErrorListeners()
	el := &errorListener{filename: filename}
	p.AddErrorListener(el)

	tree := p.CompilationUnit()

	result := &ParseResult{
		Filename: filename,
		Tree:     tree,
		Stream:   stream,
		Errors:   el.errors,
	}
	if len(el.errors) > 0 {
		return result, fmt.Errorf("parse errors in %s: %s", filename, strings.Join(el.errors, "; "))
	}
	return result, nil
}

// ParseDirectory finds and parses all .cls files under a directory.
func ParseDirectory(dir string) ([]*ParseResult, error) {
	var results []*ParseResult
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		lower := strings.ToLower(d.Name())
		if d.IsDir() || (!strings.HasSuffix(lower, ".cls") && !strings.HasSuffix(lower, ".trigger")) {
			return nil
		}
		result, parseErr := ParseFile(path)
		if result != nil {
			results = append(results, result)
		}
		if parseErr != nil {
			return parseErr
		}
		return nil
	})
	return results, err
}

// ParseSOQLString parses a SOQL query string and returns the parse tree.
// This allows runtime SOQL strings (e.g. from Database.query()) to be parsed
// with the same ANTLR grammar used for inline [SELECT ...] literals.
func ParseSOQLString(soql string) (parser.IQueryContext, error) {
	input := antlr.NewInputStream(soql)
	lexer := parser.NewApexLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	p := parser.NewApexParser(stream)
	p.RemoveErrorListeners()
	el := &errorListener{filename: "<soql>"}
	p.AddErrorListener(el)

	tree := p.Query()

	if len(el.errors) > 0 {
		return nil, fmt.Errorf("SOQL parse error: %s", strings.Join(el.errors, "; "))
	}
	return tree, nil
}

// errorListener collects parse errors.
type errorListener struct {
	*antlr.DefaultErrorListener
	filename string
	errors   []string
}

func (el *errorListener) SyntaxError(_ antlr.Recognizer, _ interface{}, line, column int, msg string, _ antlr.RecognitionException) {
	el.errors = append(el.errors, fmt.Sprintf("%s:%d:%d: %s", el.filename, line, column, msg))
}
