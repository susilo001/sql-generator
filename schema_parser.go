package sql_generator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/susilo001/sql-generator/model"
)

// FieldPartResult represents the parsed result of a single field-level tag
// part. Fields are pointers to distinguish "set this value" (non-nil) from
// "don't touch" (nil). This allows parseFieldPart to return partial updates
// that buildFieldMeta accumulates into the final FieldMeta.
type FieldPartResult struct {
	// Operators, when non-nil, replaces the operator whitelist for this
	// field. Parsed from filter:op1,op2,... tags.
	Operators map[model.Operator]bool

	// Searchable, when non-nil, sets whether this field participates in
	// global search. Parsed from search or nosearch tags.
	Searchable *bool

	// Column, when non-nil, overrides the SQL column name. Parsed from
	// column:name tags.
	Column *string

	// Name, when non-nil, overrides the query-facing field name. Parsed
	// from name:alias tags.
	Name *string

	// ExtraJoins, when non-empty, declares additional joins required by
	// this field's column expression. Parsed from joins:table1,table2,...
	// tags.
	ExtraJoins []JoinMeta
}

// JoinPartResult represents the parsed result of a single join-level tag
// part. Pointer fields use the same semantics as FieldPartResult: non-nil =
// set, nil = don't touch. IsLeftover marks tag parts that are not
// join-related and should be passed through.
type JoinPartResult struct {
	// JoinType, when non-nil, sets the join's type (LEFT or INNER). Parsed
	// from join:left or join:inner tags.
	JoinType *string

	// Table, when non-nil, sets the joined table name (and optional alias).
	// Parsed from table:name tags.
	Table *string

	// On, when non-nil, sets the join's ON condition. Parsed from on:cond
	// tags.
	On *string

	// IsLeftover marks this tag part as unrecognized by the join parser.
	// These parts are collected and returned as a concatenated string so
	// parseJoinTag can pass them back to the caller for field-level parsing.
	IsLeftover bool
}

// parseFieldPart interprets a single tag part in a field-level context
// (e.g., a tag on a struct field that declares filter operators, search
// behavior, column overrides, etc.). It returns a partial update that the
// caller accumulates into the full FieldMeta.
//
// Recognized keys: filter, search, nosearch, column, name, joins, -
// Unknown keys return an error.
func parseFieldPart(p tagPart, f reflect.StructField, table string, joinRegistry map[string]JoinMeta) (FieldPartResult, error) {
	switch p.key {

	case "filter":
		if !p.hasValue || p.value == "" {
			return FieldPartResult{}, fmt.Errorf("FromModel: field %s: filter tag needs a list, e.g. filter:eq,contains", f.Name)
		}
		ops, err := parseOperatorList(p.value)
		if err != nil {
			return FieldPartResult{}, fmt.Errorf("FromModel: field %s: filter tag %w", f.Name, err)
		}
		return FieldPartResult{Operators: ops}, nil

	case "search":
		t := true
		return FieldPartResult{Searchable: &t}, nil

	case "nosearch":
		f := false
		return FieldPartResult{Searchable: &f}, nil

	case "column":
		if !p.hasValue || p.value == "" {
			return FieldPartResult{}, fmt.Errorf("FromModel: field %s: column tag needs a name, e.g. column:type_id", f.Name)
		}
		return FieldPartResult{Column: &p.value}, nil

	case "name":
		if !p.hasValue || p.value == "" {
			return FieldPartResult{}, fmt.Errorf("FromModel: field %s: name tag needs a value, e.g. name:is_syariah", f.Name)
		}
		return FieldPartResult{Name: &p.value}, nil

	case "joins":
		if !p.hasValue || p.value == "" {
			return FieldPartResult{}, fmt.Errorf("FromModel: field %s: joins tag needs a list, e.g. joins:bond_details", f.Name)
		}
		names := strings.Split(p.value, ",")
		var extraJoins []JoinMeta
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			jm, ok := joinRegistry[n]
			if !ok {
				return FieldPartResult{}, fmt.Errorf("FromModel: field %s: joins references undeclared join %q", f.Name, n)
			}
			extraJoins = append(extraJoins, jm)
		}
		return FieldPartResult{ExtraJoins: extraJoins}, nil

	case "-":
		return FieldPartResult{}, fmt.Errorf("field excluded by tag")

	default:
		return FieldPartResult{}, fmt.Errorf("FromModel: field %s: unknown tag part %q", f.Name, p.raw)
	}
}

// parseJoinPart interprets a single tag part in a join-level context (e.g.,
// a tag on a nested struct field that declares a SQL join). It returns a
// partial update that the caller accumulates into the full JoinMeta, or sets
// IsLeftover=true if the tag part is not join-related.
//
// Recognized keys: join, table, on
// Unrecognized keys are marked as leftovers for the caller to handle.
func parseJoinPart(p tagPart) (JoinPartResult, error) {
	switch p.key {

	case "join":
		switch strings.ToLower(p.value) {
		case "", "left":
			t := "LEFT"
			return JoinPartResult{JoinType: &t}, nil
		case "inner":
			t := "INNER"
			return JoinPartResult{JoinType: &t}, nil
		default:
			return JoinPartResult{}, fmt.Errorf("unknown join type %q (want left or inner)", p.value)
		}

	case "table":
		if !p.hasValue || p.value == "" {
			return JoinPartResult{}, fmt.Errorf("table: requires a non-empty value")
		}
		return JoinPartResult{Table: &p.value}, nil

	case "on":
		if !p.hasValue || p.value == "" {
			return JoinPartResult{}, fmt.Errorf("on: requires a non-empty value")
		}
		return JoinPartResult{On: &p.value}, nil

	default:
		// Not a join-related tag part; caller should handle it as a field tag
		return JoinPartResult{IsLeftover: true}, nil
	}
}


