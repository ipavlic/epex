package tracer

import (
	"sort"
	"time"
)

// MethodStats holds aggregate statistics for a single method.
type MethodStats struct {
	Class     string        `json:"class"`
	Method    string        `json:"method"`
	Calls     int           `json:"calls"`
	TotalTime time.Duration `json:"-"`
	TotalMs   float64       `json:"totalMs"`
	AvgMs     float64       `json:"avgMs"`
}

// SOQLStats holds aggregate statistics for a single SOQL query pattern.
type SOQLStats struct {
	Query     string        `json:"query"`
	Calls     int           `json:"calls"`
	TotalRows int           `json:"totalRows"`
	TotalTime time.Duration `json:"-"`
	TotalMs   float64       `json:"totalMs"`
}

// DMLStats holds aggregate statistics for a DML operation type on an SObject.
type DMLStats struct {
	Operation string        `json:"operation"`
	SObject   string        `json:"sobject"`
	Calls     int           `json:"calls"`
	TotalRows int           `json:"totalRows"`
	TotalTime time.Duration `json:"-"`
	TotalMs   float64       `json:"totalMs"`
}

// LineStats holds execution count for a source line.
type LineStats struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Executions int    `json:"executions"`
}

// Summary holds all aggregate statistics from a trace.
type Summary struct {
	Methods []MethodStats `json:"methods"`
	SOQL    []SOQLStats   `json:"soql"`
	DML     []DMLStats    `json:"dml"`
	Lines   []LineStats   `json:"lines"`
}

// BuildSummary aggregates trace events into a Summary.
func BuildSummary(events []TraceEvent, topLines int) Summary {
	s := Summary{}

	// Methods
	type methodKey struct{ class, method string }
	methods := make(map[methodKey]*MethodStats)
	for _, e := range events {
		if e.Type != EventMethodExit {
			continue
		}
		k := methodKey{e.Class, e.Method}
		ms, ok := methods[k]
		if !ok {
			ms = &MethodStats{Class: e.Class, Method: e.Method}
			methods[k] = ms
		}
		ms.Calls++
		ms.TotalTime += e.Duration
	}
	for _, ms := range methods {
		ms.TotalMs = float64(ms.TotalTime.Microseconds()) / 1000.0
		if ms.Calls > 0 {
			ms.AvgMs = ms.TotalMs / float64(ms.Calls)
		}
		s.Methods = append(s.Methods, *ms)
	}
	sort.Slice(s.Methods, func(i, j int) bool {
		return s.Methods[i].TotalMs > s.Methods[j].TotalMs
	})

	// SOQL
	soql := make(map[string]*SOQLStats)
	for _, e := range events {
		if e.Type != EventSOQL {
			continue
		}
		ss, ok := soql[e.Detail]
		if !ok {
			ss = &SOQLStats{Query: e.Detail}
			soql[e.Detail] = ss
		}
		ss.Calls++
		ss.TotalRows += e.RowCount
		ss.TotalTime += e.Duration
	}
	for _, ss := range soql {
		ss.TotalMs = float64(ss.TotalTime.Microseconds()) / 1000.0
		s.SOQL = append(s.SOQL, *ss)
	}
	sort.Slice(s.SOQL, func(i, j int) bool {
		return s.SOQL[i].TotalMs > s.SOQL[j].TotalMs
	})

	// DML
	type dmlKey struct{ op, sobject string }
	dml := make(map[dmlKey]*DMLStats)
	for _, e := range events {
		if e.Type != EventDML {
			continue
		}
		k := dmlKey{e.Detail, e.Class}
		ds, ok := dml[k]
		if !ok {
			ds = &DMLStats{Operation: e.Detail, SObject: e.Class}
			dml[k] = ds
		}
		ds.Calls++
		ds.TotalRows += e.RowCount
		ds.TotalTime += e.Duration
	}
	for _, ds := range dml {
		ds.TotalMs = float64(ds.TotalTime.Microseconds()) / 1000.0
		s.DML = append(s.DML, *ds)
	}
	sort.Slice(s.DML, func(i, j int) bool {
		return s.DML[i].TotalMs > s.DML[j].TotalMs
	})

	// Line heat map
	type lineKey struct {
		file string
		line int
	}
	lines := make(map[lineKey]int)
	for _, e := range events {
		if e.Type != EventLine {
			continue
		}
		k := lineKey{e.File, e.Line}
		lines[k]++
	}
	for k, count := range lines {
		s.Lines = append(s.Lines, LineStats{File: k.file, Line: k.line, Executions: count})
	}
	sort.Slice(s.Lines, func(i, j int) bool {
		return s.Lines[i].Executions > s.Lines[j].Executions
	})
	if topLines > 0 && len(s.Lines) > topLines {
		s.Lines = s.Lines[:topLines]
	}

	return s
}
