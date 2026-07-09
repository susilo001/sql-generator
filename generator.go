// Package sql_generator translates a frontend-friendly, JSON-serializable
// model.Query into a slice of composable GORM scopes:
//
//	scopes, err := gen.Scopes(query)
//	db.Scopes(scopes...).Find(&results)
//
// The Generator never builds SQL directly against user-supplied field names
// or table names: every field referenced in a Query must be declared in the
// Generator's ModelMeta schema, and every operator must be explicitly
// whitelisted per field. Fields that are not declared are rejected with an
// error rather than silently ignored, with the exception of unknown Sort
// and Select fields (see sortScope and selectScope), which are skipped.
//
// All comparison values are passed to GORM as parameterized query arguments
// (placeholders), never interpolated into the SQL string, so ordinary SQL
// injection via filter, sort, or search values is not possible as long as
// ModelMeta.Column, JoinMeta.Table, and JoinMeta.On are populated from
// trusted, developer-authored configuration rather than user input.
package sql_generator

import (
	"fmt"
	"strings"

	"github.com/susilo001/sql-generator/model"

	"gorm.io/gorm"
)

// ModelMeta is the schema definition for a single GORM model: the set of
// fields a Query is allowed to reference, keyed by the field name used in
// model.Query (which may differ from the underlying column name).
//
// Any field name that appears in a Query but is not a key in Fields causes
// the relevant scope builder to reject the query (for Filters and
// FilterGroups) or silently skip that entry (for Sorts and
// SelectParameter.Fields).
type ModelMeta struct {
	// Fields maps a query-facing field name (e.g. "user_name") to its
	// column, join, and operator metadata. Keys are matched exactly against
	// Filter.FieldName, Sort.FieldName, and SelectParameter.Fields entries.
	Fields map[string]FieldMeta
}

// FieldMeta describes how a single query-facing field maps to the
// underlying SQL column, which filter operators are permitted against it,
// and whether it participates in global search.
type FieldMeta struct {
	// Column is the SQL column reference used when building WHERE, ORDER BY,
	// and SELECT clauses (e.g. "status" or "users.name" for a joined
	// table). This value is concatenated directly into SQL strings, so it
	// must only ever come from trusted, developer-authored ModelMeta
	// configuration — never from user or request input.
	Column string

	// Join, if non-nil, declares a join that is automatically applied
	// whenever this field is referenced in Filters, Sorts, or
	// SelectParameter.Fields. See JoinMeta and the joinScope builder.
	Join *JoinMeta

	// Searchable marks this field as a target for the global
	// model.Query.Search term. When true, the field's Column is included in
	// a LIKE/ILIKE OR clause alongside every other Searchable field (see
	// Generator.CaseSensitiveSearch for the LIKE/ILIKE choice).
	Searchable bool

	// Operators whitelists which model.Operator values are permitted for
	// this field. A Filter referencing an operator not present (or set to
	// false) in this map is rejected with an error. There is no implicit
	// default: every operator a client may use against a field must be
	// explicitly listed here.
	Operators map[model.Operator]bool
}

// JoinMeta declares a SQL join required to resolve a field that lives on a
// related table. Table and On are concatenated directly into the generated
// SQL and must only come from trusted, developer-authored configuration.
type JoinMeta struct {
	// Table is the joined table name (and optional alias), e.g. "users" or
	// "users u".
	Table string

	// On is the raw join condition, e.g. "users.id = orders.user_id".
	On string

	// Type is the join keyword, "LEFT" or "INNER". Defaults to "LEFT" when
	// empty.
	Type string
}

// Generator compiles a model.Query into GORM scopes according to a fixed
// ModelMeta schema. A Generator is safe for concurrent use once
// constructed: it is read-only and holds no per-call state — all Query
// input is passed as a parameter to Scopes, never stored on the Generator
// itself.
type Generator struct {
	// Schema is the field whitelist and join metadata every Query is
	// validated against. Required: Scopes dereferences Schema and will
	// panic with a nil-pointer error if it is left nil.
	Schema *ModelMeta

	// DefaultFieldForSort is the column used to order results when the
	// incoming Query specifies no Sorts. Should reference a stable, ideally
	// indexed column (e.g. a primary key or a "created_at" timestamp).
	DefaultFieldForSort string

	// CaseSensitiveSearch controls the operator used by the global search
	// scope only; it has no effect on individual filter operators (IsContain,
	// IsBeginWith, IsEndWith always use plain LIKE). When false (the zero
	// value), search uses ILIKE, which is PostgreSQL-specific — other SQL
	// dialects do not support ILIKE and will return a driver error. When
	// true, search uses standard LIKE.
	CaseSensitiveSearch bool

	// MaxFiltersPerQuery caps the total number of Filter entries allowed in
	// a single Query, counted recursively through all FilterGroups. Zero
	// (the default) means unlimited. Use this to bound query-building cost
	// and guard against abusive or malformed requests.
	MaxFiltersPerQuery int

	// MaxSortsPerQuery caps the number of Sort entries allowed in a single
	// Query. Zero (the default) means unlimited.
	MaxSortsPerQuery int
}

// Scopes compiles q into an ordered slice of GORM scopes according to g's
// schema, ready to be passed to db.Scopes(scopes...). The returned scopes
// apply, in order:
//
//  1. Flat Filters (ANDed together) — see filterScopes.
//  2. FilterGroups (nested AND/OR) — see filterGroupScopes.
//  3. Joins required by any referenced field — see joinScope.
//  4. Global Search across Searchable fields — see searchScope.
//  5. Sorts, or DefaultFieldForSort when none are given — see sortScope.
//  6. Field projection (SELECT) — see selectScope.
//  7. Preload (eager loading) — see preloadScope.
//  8. Pagination (LIMIT/OFFSET) — see paginationScope.
//
// Scopes returns an error, without applying any scope, if q exceeds
// MaxFiltersPerQuery or MaxSortsPerQuery, if a Filter or FilterGroup
// references a field not declared in g.Schema, if a Filter's Operator is not
// whitelisted for its field, or if an IS_BETWEEN filter does not carry
// exactly two RangeValues. Unknown fields in Sorts and SelectParameter.Fields
// are not errors — they are silently skipped (see sortScope, selectScope).
func (g *Generator) Scopes(q model.Query) ([]func(*gorm.DB) *gorm.DB, error) {
	var scopes []func(*gorm.DB) *gorm.DB

	// Apply query limits validation
	if err := g.validateQuery(q); err != nil {
		return nil, err
	}

	fs, err := g.filterScopes(q)
	if err != nil {
		return nil, err
	}
	scopes = append(scopes, fs...)

	// Handle filter groups
	fgs, err := g.filterGroupScopes(q)
	if err != nil {
		return nil, err
	}
	scopes = append(scopes, fgs...)

	scopes = append(scopes, g.joinScope(q), g.searchScope(q), g.sortScope(q), g.selectScope(q), g.preloadScope(q), g.paginationScope(q))

	return scopes, nil
}

// filterScopes compiles q.SelectParameter.Filters into one GORM scope per
// Filter, all of which are ultimately ANDed together by GORM's default
// chaining of Where calls. Every Filter's FieldName must be a key in
// g.Schema.Fields and its Operator must be set to true in that field's
// FieldMeta.Operators, or the whole call returns an error and no scopes.
// IS_BETWEEN filters are validated to carry exactly two RangeValues before
// any scope is built, so a malformed filter fails fast rather than at query
// execution time.
func (g *Generator) filterScopes(q model.Query) ([]func(*gorm.DB) *gorm.DB, error) {
	var scopes []func(*gorm.DB) *gorm.DB

	for _, f := range q.SelectParameter.Filters {
		meta, ok := g.Schema.Fields[f.FieldName]
		if !ok {
			return nil, fmt.Errorf("unknown field: %s", f.FieldName)
		}

		if !meta.Operators[f.Operator] {
			return nil, fmt.Errorf("operator not allowed: %s", f.Operator)
		}

		col := meta.Column
		op := f.Operator
		val := f.Value
		rng := f.RangeValues

		// Validate BEFORE creating scope
		if op == model.IsBetween && len(rng) != 2 {
			return nil, fmt.Errorf("IS_BETWEEN requires exactly 2 values")
		}

		scope := func(db *gorm.DB) *gorm.DB {
			switch op {

			case model.IsEqual:
				return db.Where(col+" = ?", val)

			case model.IsNotEqual:
				return db.Where(col+" <> ?", val)

			case model.IsLessThan:
				return db.Where(col+" < ?", val)

			case model.IsMoreThan:
				return db.Where(col+" > ?", val)

			case model.IsLessThanOrEqual:
				return db.Where(col+" <= ?", val)

			case model.IsMoreThanOrEqual:
				return db.Where(col+" >= ?", val)

			case model.IsContain:
				return db.Where(col+" LIKE ?", "%"+fmt.Sprint(val)+"%")

			case model.IsBeginWith:
				return db.Where(col+" LIKE ?", fmt.Sprint(val)+"%")

			case model.IsEndWith:
				return db.Where(col+" LIKE ?", "%"+fmt.Sprint(val))

			case model.IsBetween:
				return db.Where(col+" BETWEEN ? AND ?", rng[0], rng[1])

			case model.IsIn:
				return db.Where(col+" IN ?", rng)

			case model.IsNotIn:
				return db.Where(col+" NOT IN ?", rng)

			case model.IsNull:
				return db.Where(col + " IS NULL")

			case model.IsNotNull:
				return db.Where(col + " IS NOT NULL")

			default:
				// unreachable due to earlier validation
				return db
			}
		}

		scopes = append(scopes, scope)
	}

	return scopes, nil
}

// searchScope applies q.Search, if non-empty, as a single OR-joined
// LIKE/ILIKE clause across every field in g.Schema.Fields marked Searchable.
// The operator is ILIKE (PostgreSQL-only, case-insensitive) by default, or
// LIKE when g.CaseSensitiveSearch is true. If no field is Searchable, this
// scope is a no-op regardless of q.Search.
func (g *Generator) searchScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if q.Search == "" {
			return db
		}

		term := "%" + q.Search + "%"

		var ors []string
		var args []any

		likeOp := "LIKE"
		if !g.CaseSensitiveSearch {
			likeOp = "ILIKE" // PostgreSQL case-insensitive
		}

		for _, f := range g.Schema.Fields {
			if f.Searchable {
				ors = append(ors, f.Column+" "+likeOp+" ?")
				args = append(args, term)
			}
		}

		if len(ors) > 0 {
			return db.Where("("+strings.Join(ors, " OR ")+")", args...)
		}

		return db
	}
}

// sortScope applies q.SelectParameter.Sorts as one ORDER BY clause per
// entry, in the order given, ASC or DESC per Sort.SortDirection. When no
// Sorts are given, it falls back to ordering by g.DefaultFieldForSort
// ascending. Unlike filterScopes, a Sort referencing a field not present in
// g.Schema.Fields is not an error — it is silently skipped, so callers
// relying on a specific sort order should validate FieldName themselves if
// silent omission is unacceptable.
func (g *Generator) sortScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(q.SelectParameter.Sorts) == 0 {
			return db.Order(g.DefaultFieldForSort + " ASC")
		}

		for _, s := range q.SelectParameter.Sorts {
			meta, ok := g.Schema.Fields[s.FieldName]
			if !ok {
				continue
			}

			dir := "ASC"
			if s.SortDirection == model.Descending {
				dir = "DESC"
			}

			db = db.Order(meta.Column + " " + dir)
		}

		return db
	}
}

// paginationScope applies LIMIT/OFFSET derived from
// q.SelectParameter.PageDescriptor. Page defaults to 1 and PageSize defaults
// to 20 when unset or less than 1. There is no upper bound on PageSize —
// callers that need to cap page size to prevent large-result-set abuse must
// enforce that themselves before calling Scopes.
func (g *Generator) paginationScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		p := q.SelectParameter.PageDescriptor.Page
		s := q.SelectParameter.PageDescriptor.PageSize

		if p < 1 {
			p = 1
		}
		if s < 1 {
			s = 20
		}

		return db.Limit(s).Offset((p - 1) * s)
	}
}

// validateQuery enforces g.MaxFiltersPerQuery and g.MaxSortsPerQuery against
// q, counting Filters recursively through all FilterGroups via
// countFiltersInGroup. A zero limit means unlimited (the default). It
// returns an error describing which limit was exceeded, or nil if q is
// within bounds.
func (g *Generator) validateQuery(q model.Query) error {
	if g.MaxFiltersPerQuery > 0 {
		totalFilters := len(q.SelectParameter.Filters)
		for _, fg := range q.SelectParameter.FilterGroups {
			totalFilters += g.countFiltersInGroup(fg)
		}
		if totalFilters > g.MaxFiltersPerQuery {
			return fmt.Errorf("too many filters: %d (max: %d)", totalFilters, g.MaxFiltersPerQuery)
		}
	}

	if g.MaxSortsPerQuery > 0 && len(q.SelectParameter.Sorts) > g.MaxSortsPerQuery {
		return fmt.Errorf("too many sorts: %d (max: %d)", len(q.SelectParameter.Sorts), g.MaxSortsPerQuery)
	}

	return nil
}

// countFiltersInGroup returns the total number of Filter entries in fg,
// including all Filters in every nested FilterGroup, recursively.
func (g *Generator) countFiltersInGroup(fg model.FilterGroup) int {
	count := len(fg.Filters)
	for _, nested := range fg.FilterGroups {
		count += g.countFiltersInGroup(nested)
	}
	return count
}

// filterGroupScopes builds one GORM scope per top-level FilterGroup in q.
// Each scope applies a single, fully parenthesized WHERE clause built by
// buildFilterGroupScope, so nested AND/OR structure is preserved exactly as
// declared.
func (g *Generator) filterGroupScopes(q model.Query) ([]func(*gorm.DB) *gorm.DB, error) {
	var scopes []func(*gorm.DB) *gorm.DB

	for _, fg := range q.SelectParameter.FilterGroups {
		scope, err := g.buildFilterGroupScope(fg)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}

	return scopes, nil
}

// buildFilterGroupScope compiles fg into a single GORM scope that applies
// one parenthesized `db.Where(sql, args...)` call. It is a thin wrapper
// around buildFilterGroupSQL, which does the actual recursive SQL building.
func (g *Generator) buildFilterGroupScope(fg model.FilterGroup) (func(*gorm.DB) *gorm.DB, error) {
	sql, args, err := g.buildFilterGroupSQL(fg)
	if err != nil {
		return nil, err
	}

	return func(db *gorm.DB) *gorm.DB {
		if sql == "" {
			return db
		}
		return db.Where(sql, args...)
	}, nil
}

// buildFilterGroupSQL recursively compiles a FilterGroup into a single
// parenthesized SQL fragment and its bound arguments.
//
// Unlike a naive implementation that applies nested groups as independent
// `db.Where(...)` calls (which always ANDs them onto the parent regardless of
// the nested group's own Condition), this builds nested groups as SQL
// sub-fragments first and folds them into the parent's condition list. This
// preserves nesting semantics, e.g.:
//
//	FilterGroup{Condition: OR, Filters: [A, B], FilterGroups: [
//	    {Condition: AND, Filters: [C, D]},
//	]}
//
// compiles to: (A OR B OR (C AND D))
//
// Unknown fields and disallowed operators return an error rather than being
// silently skipped, matching the behavior of filterScopes for top-level
// Filters.
func (g *Generator) buildFilterGroupSQL(fg model.FilterGroup) (string, []any, error) {
	var conditions []string
	var args []any

	// Individual filters in this group.
	for _, f := range fg.Filters {
		meta, ok := g.Schema.Fields[f.FieldName]
		if !ok {
			return "", nil, fmt.Errorf("unknown field: %s", f.FieldName)
		}

		if !meta.Operators[f.Operator] {
			return "", nil, fmt.Errorf("operator not allowed: %s", f.Operator)
		}

		if f.Operator == model.IsBetween && len(f.RangeValues) != 2 {
			return "", nil, fmt.Errorf("IS_BETWEEN requires exactly 2 values")
		}

		condition, filterArgs := g.buildFilterCondition(meta.Column, f)
		if condition != "" {
			conditions = append(conditions, condition)
			args = append(args, filterArgs...)
		}
	}

	// Nested groups are compiled to their own parenthesized fragment and
	// folded into this group's condition list, so the parent's AND/OR joiner
	// applies uniformly across sibling filters and sibling sub-groups.
	for _, nested := range fg.FilterGroups {
		nestedSQL, nestedArgs, err := g.buildFilterGroupSQL(nested)
		if err != nil {
			return "", nil, err
		}
		if nestedSQL != "" {
			conditions = append(conditions, nestedSQL)
			args = append(args, nestedArgs...)
		}
	}

	if len(conditions) == 0 {
		return "", nil, nil
	}

	joiner := " AND "
	if fg.Condition == model.Or {
		joiner = " OR "
	}

	return "(" + strings.Join(conditions, joiner) + ")", args, nil
}

// buildFilterCondition returns the parameterized SQL fragment and its bound
// arguments for a single Filter against the already-resolved column col.
// This is the single source of truth for operator-to-SQL translation shared
// by both buildFilterGroupSQL (nested groups) and, in spirit, filterScopes
// (flat filters use an equivalent inline switch). Callers are expected to
// have already validated f.Operator against the field's whitelist and, for
// IS_BETWEEN, that f.RangeValues has exactly two elements — this function
// does not re-validate and returns ("", nil) for an IS_BETWEEN filter with
// the wrong number of RangeValues, or for any operator it does not
// recognize.
func (g *Generator) buildFilterCondition(col string, f model.Filter) (string, []any) {
	switch f.Operator {
	case model.IsEqual:
		return col + " = ?", []any{f.Value}
	case model.IsNotEqual:
		return col + " <> ?", []any{f.Value}
	case model.IsLessThan:
		return col + " < ?", []any{f.Value}
	case model.IsMoreThan:
		return col + " > ?", []any{f.Value}
	case model.IsLessThanOrEqual:
		return col + " <= ?", []any{f.Value}
	case model.IsMoreThanOrEqual:
		return col + " >= ?", []any{f.Value}
	case model.IsContain:
		return col + " LIKE ?", []any{"%" + fmt.Sprint(f.Value) + "%"}
	case model.IsBeginWith:
		return col + " LIKE ?", []any{fmt.Sprint(f.Value) + "%"}
	case model.IsEndWith:
		return col + " LIKE ?", []any{"%" + fmt.Sprint(f.Value)}
	case model.IsBetween:
		if len(f.RangeValues) == 2 {
			return col + " BETWEEN ? AND ?", []any{f.RangeValues[0], f.RangeValues[1]}
		}
	case model.IsIn:
		return col + " IN ?", []any{f.RangeValues}
	case model.IsNotIn:
		return col + " NOT IN ?", []any{f.RangeValues}
	case model.IsNull:
		return col + " IS NULL", nil
	case model.IsNotNull:
		return col + " IS NOT NULL", nil
	}
	return "", nil
}

// joinScope inspects every field referenced by q (in Filters, Sorts, and
// SelectParameter.Fields) and applies the JoinMeta declared for each one
// that has a non-nil Join, deduplicating by Table+On so a join used by
// multiple fields is only applied once. The join keyword defaults to LEFT
// when JoinMeta.Type is empty. Fields not present in g.Schema.Fields, or
// present but with a nil Join, are ignored here (they are validated, if at
// all, by the scope that consumes them — filterScopes, sortScope, or
// selectScope).
func (g *Generator) joinScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		appliedJoins := make(map[string]bool)

		// Collect all fields used in query
		allFields := make(map[string]bool)
		for _, f := range q.SelectParameter.Filters {
			allFields[f.FieldName] = true
		}
		for _, s := range q.SelectParameter.Sorts {
			allFields[s.FieldName] = true
		}
		for _, field := range q.SelectParameter.Fields {
			allFields[field] = true
		}

		// Apply necessary joins
		for fieldName := range allFields {
			if meta, ok := g.Schema.Fields[fieldName]; ok && meta.Join != nil {
				joinKey := meta.Join.Table + meta.Join.On
				if !appliedJoins[joinKey] {
					joinType := meta.Join.Type
					if joinType == "" {
						joinType = "LEFT"
					}
					db = db.Joins(joinType + " JOIN " + meta.Join.Table + " ON " + meta.Join.On)
					appliedJoins[joinKey] = true
				}
			}
		}

		return db
	}
}

// selectScope applies db.Select on the columns resolved from
// q.SelectParameter.Fields, restricting the query to those columns instead
// of SELECT *. When Fields is empty, this scope is a no-op. Like sortScope,
// a field name not present in g.Schema.Fields is silently skipped rather
// than treated as an error.
func (g *Generator) selectScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(q.SelectParameter.Fields) == 0 {
			return db
		}

		var columns []string
		for _, fieldName := range q.SelectParameter.Fields {
			if meta, ok := g.Schema.Fields[fieldName]; ok {
				columns = append(columns, meta.Column)
			}
		}

		if len(columns) > 0 {
			return db.Select(columns)
		}

		return db
	}
}

// preloadScope applies db.Preload for every entry in
// q.SelectParameter.Preloads, enabling GORM eager loading of the named
// associations. Preload names are passed through to GORM as-is and are not
// validated against g.Schema — an invalid association name will fail at
// GORM's query-execution time, not here.
func (g *Generator) preloadScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		for _, preload := range q.SelectParameter.Preloads {
			db = db.Preload(preload)
		}
		return db
	}
}
