package sql_generator

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/susilo001/sql-generator/model"

	gormschema "gorm.io/gorm/schema"
)

// TagKey is the struct tag key read by FromModel, e.g.
//
//	Name string `sqlgen:"filter:eq,contains;search"`
const TagKey = "sqlgen"

// tagPart is one ";"-separated, "key:value" segment of a parsed sqlgen
// tag. hasValue distinguishes a bare key with no ":" (hasValue false)
// from a key with an explicit, possibly empty, value (hasValue true).
type tagPart struct {
	key      string
	value    string
	hasValue bool
	raw      string
}

// splitTagParts tokenizes a sqlgen tag into its ";"-separated parts,
// trimming whitespace and lowercasing each key. It is the single shared
// tokenizer used by both buildFieldMeta and parseJoinTag, so the tag
// grammar's low-level syntax (separators, trimming, case-folding) only
// has one implementation to keep in sync.
func splitTagParts(tag string) []tagPart {
	var parts []tagPart
	for _, raw := range strings.Split(tag, ";") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		key, value, hasValue := strings.Cut(raw, ":")
		parts = append(parts, tagPart{
			key:      strings.ToLower(strings.TrimSpace(key)),
			value:    strings.TrimSpace(value),
			hasValue: hasValue,
			raw:      raw,
		})
	}
	return parts
}

// Options carries the Generator settings that cannot be derived from the
// model struct itself. It mirrors the tunable fields on Generator; see the
// corresponding Generator field documentation for the semantics of each.
// The zero value is valid: no default sort field, ILIKE search, and no
// limits.
type Options struct {
	// Table, when set, qualifies every root-level column with the table
	// name (e.g. Table: "investment_products" turns column "id" into
	// "investment_products.id"). Recommended whenever the schema declares
	// joins, so base-table columns cannot collide with joined columns.
	// Columns overridden via the `column:` tag part are used verbatim and
	// never qualified.
	Table string

	// DefaultFieldForSort is copied to Generator.DefaultFieldForSort.
	DefaultFieldForSort string

	// CaseSensitiveSearch is copied to Generator.CaseSensitiveSearch.
	CaseSensitiveSearch bool

	// MaxFiltersPerQuery is copied to Generator.MaxFiltersPerQuery.
	MaxFiltersPerQuery int

	// MaxSortsPerQuery is copied to Generator.MaxSortsPerQuery.
	MaxSortsPerQuery int

	// MaxFilterGroupDepth is copied to Generator.MaxFilterGroupDepth.
	MaxFilterGroupDepth int

	// MaxFilterGroupsPerQuery is copied to
	// Generator.MaxFilterGroupsPerQuery.
	MaxFilterGroupsPerQuery int

	// MaxPageSize is copied to Generator.MaxPageSize.
	MaxPageSize int

	// MaxRangeValues is copied to Generator.MaxRangeValues.
	MaxRangeValues int

	// AllowedPreloads is copied to ModelMeta.AllowedPreloads.
	AllowedPreloads map[string]bool
}

// operatorAliases maps the short operator names accepted in a `sqlgen`
// filter list to their model.Operator constants.
var operatorAliases = map[string]model.Operator{
	"eq":         model.IsEqual,
	"ne":         model.IsNotEqual,
	"gt":         model.IsMoreThan,
	"gte":        model.IsMoreThanOrEqual,
	"lt":         model.IsLessThan,
	"lte":        model.IsLessThanOrEqual,
	"contains":   model.IsContain,
	"startswith": model.IsBeginWith,
	"endswith":   model.IsEndWith,
	"between":    model.IsBetween,
	"in":         model.IsIn,
	"notin":      model.IsNotIn,
	"null":       model.IsNull,
	"notnull":    model.IsNotNull,
}

// operatorPresets maps preset names, usable in a `sqlgen` filter list, to
// groups of operators. "all" expands to every operator, "comparable" to
// the equality/ordering/range operators, and "text" to the equality and
// LIKE-style operators.
var operatorPresets = map[string][]model.Operator{
	// "all" is derived from operatorAliases below (in an init func) rather
	// than hand-listed here, so the two can never drift out of sync when a
	// new operator alias is added.
	"comparable": {
		model.IsEqual, model.IsNotEqual,
		model.IsLessThan, model.IsMoreThan,
		model.IsLessThanOrEqual, model.IsMoreThanOrEqual,
		model.IsBetween,
	},
	"text": {
		model.IsEqual,
		model.IsContain, model.IsBeginWith, model.IsEndWith,
	},
}

// init derives the all preset from operatorAliases so the two lists
// cannot drift: every operator alias is, by construction, part of all.
func init() {
	all := make([]model.Operator, 0, len(operatorAliases))
	for _, op := range operatorAliases {
		all = append(all, op)
	}
	operatorPresets["all"] = all
}

// FromModel builds a Generator by reflecting over a model struct, so the
// ModelMeta schema never has to be written by hand. Pass the struct (or a
// pointer to it) that the repository layer already uses with GORM:
//
//	type Product struct {
//		ID         int        `gorm:"column:id"`
//		Name       string     `sqlgen:"filter:text;search"`
//		TypeID     int        `sqlgen:"filter:eq,ne,in"`
//		DataStatus string     `gorm:"column:data_status"`
//		Internal   string     `sqlgen:"-"`
//	}
//
//	gen, err := sql_generator.FromModel(&Product{}, sql_generator.Options{
//		DefaultFieldForSort: "id",
//		MaxFiltersPerQuery:  50,
//	})
//
// Default behavior (no `sqlgen` tag on a field): the field is included
// and fully open — every filter operator is allowed, and the field is
// sortable and selectable. String fields are additionally included in
// global search by default; non-string fields are not, because the
// generated search clause uses LIKE/ILIKE, which is invalid against
// non-text columns. Tags therefore only ever restrict or adjust:
//
//	filter:eq,ne,gt,gte,lt,lte,contains,startswith,endswith,between,in,notin,null,notnull
//	        — restrict the operator whitelist to the listed aliases.
//	          Presets may be mixed in: "all", "comparable"
//	          (eq,ne,gt,gte,lt,lte,between), "text"
//	          (eq,contains,startswith,endswith).
//	search  — force the field into global search (e.g. a non-string
//	          column stored as text).
//	nosearch — force the field out of global search.
//	column:name — override the SQL column name.
//	-       — exclude the field from the schema entirely.
//
// Tag parts are separated by ';', list items by ','. Column name
// resolution order: `sqlgen` column: override, then `gorm:"column:..."`,
// then GORM's snake_case of the Go field name (ID → id, TypeID →
// type_id). The resolved column name is also the query-facing field name
// clients use in filters and sorts.
//
// Additional tag parts for real-world repository schemas:
//
//	name:query_name — override the query-facing field name clients use
//	          in filters/sorts (defaults to the unqualified column name).
//	          Use it when the SQL column is an expression or when two
//	          joined tables share a column name.
//
// When Options.Table is set, every root-level column is qualified with
// it ("id" → "investment_products.id") while the query-facing field name
// stays unqualified ("id"). `column:` overrides are used verbatim — they
// may be a qualified name or a full SQL expression such as
// "COALESCE(a.x, b.x)" — and are never re-qualified.
//
// Joined tables are declared as nested struct fields carrying a join tag:
//
//	type InvestmentProduct struct {
//		ID     int
//		Name   string `sqlgen:"filter:text;search"`
//		Fund   MutualFundDetail `sqlgen:"join:left;table:mutual_fund_details;on:mutual_fund_details.product_id = investment_products.id"`
//		Bond   *BondDetail      `sqlgen:"join:left;table:bond_details;on:bond_details.product_id = investment_products.id"`
//	}
//
//	type MutualFundDetail struct {
//		FundCategory      string `sqlgen:"filter:eq,in,contains,null,notnull;search"`
//		InvestmentManager string `sqlgen:"filter:eq,contains,startswith;search"`
//	}
//
// Every leaf field inside a joined struct is qualified with the join's
// table (alias if one is given, e.g. "mutual_fund_details f" → "f") and
// carries a JoinMeta, so the Generator auto-applies the JOIN whenever the
// field is referenced. join accepts "left" or "inner" (default "left");
// table: and on: are required alongside it and are concatenated into the
// SQL — they are developer-authored configuration, never request input.
//
// A leaf field whose column: expression needs a sibling join may add
// `joins:table_or_alias,...`. Each value must identify a table: declared
// by a join-tagged struct field. FromModel attaches those joins in addition
// to the leaf field's enclosing Join.
//
// Embedded (anonymous) structs such as gorm.Model are recursed into;
// unexported fields are skipped. Named struct fields WITHOUT a join tag
// are included as leaf columns only when they are database-scalar types
// (time.Time, gorm.DeletedAt, sql.NullString, driver.Valuer
// implementations, ...); plain association structs/slices without a join
// tag are skipped, since they have no single column.
//
// FromModel fails fast: a malformed tag (unknown operator alias, unknown
// tag part, empty column/name override, join missing table/on, join tag
// on a non-struct field) or two fields resolving to the same query-facing
// name return an error rather than being silently skipped.
func FromModel(m any, opts ...Options) (*Generator, error) {
	t := reflect.TypeOf(m)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("FromModel: expected a struct or pointer to struct, got %T", m)
	}

	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}

	joinRegistry := map[string]JoinMeta{}
	collectJoins(t, joinRegistry)

	fields := map[string]FieldMeta{}
	if err := collectFields(t, fields, o.Table, nil, joinRegistry); err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("FromModel: %s has no usable fields", t.Name())
	}

	return &Generator{
		Schema: &ModelMeta{
			Fields:          fields,
			AllowedPreloads: o.AllowedPreloads,
		},
		DefaultFieldForSort:     o.DefaultFieldForSort,
		CaseSensitiveSearch:     o.CaseSensitiveSearch,
		MaxFiltersPerQuery:      o.MaxFiltersPerQuery,
		MaxSortsPerQuery:        o.MaxSortsPerQuery,
		MaxFilterGroupDepth:     o.MaxFilterGroupDepth,
		MaxFilterGroupsPerQuery: o.MaxFilterGroupsPerQuery,
		MaxPageSize:             o.MaxPageSize,
		MaxRangeValues:          o.MaxRangeValues,
	}, nil
}

// collectJoins walks t and records every join declared by a nested
// struct's join tag into registry, keyed by the join's table name (the
// qualifier before any alias, e.g. "mutual_fund_details"). It runs as a
// separate pass before collectFields so a leaf field's `joins:` tag part
// (see buildFieldMeta) can reference a join declared anywhere else in the
// struct, regardless of field order. Malformed join tags are ignored here;
// collectFields reports them.
func collectJoins(t reflect.Type, registry map[string]JoinMeta) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		if f.Anonymous {
			ft := derefType(f.Type)
			if ft.Kind() == reflect.Struct {
				collectJoins(ft, registry)
				continue
			}
		}

		if !f.IsExported() {
			continue
		}

		tag := f.Tag.Get(TagKey)
		if tag == "-" {
			continue
		}

		jm, _, ok, err := parseJoinTag(f, tag)
		if err != nil || !ok {
			continue
		}
		registry[joinQualifier(jm.Table)] = *jm
		if tableName := strings.Fields(jm.Table); len(tableName) > 0 {
			registry[tableName[0]] = *jm
		}

		ft := derefType(f.Type)
		if ft.Kind() == reflect.Struct {
			collectJoins(ft, registry)
		}
	}
}

// collectFields walks the struct type t and adds one FieldMeta per usable
// field to out, recursing into embedded (anonymous) structs and into
// nested structs carrying a join tag. table qualifies plain column names
// ("" leaves them unqualified); join, when non-nil, is attached to every
// leaf field collected within that joined struct. joinRegistry resolves a
// leaf field's `joins:` tag part (extra joins beyond the enclosing struct's
// own join) to their declared JoinMeta. It returns an error on the first
// malformed tag or duplicate query-facing name.
func collectFields(t reflect.Type, out map[string]FieldMeta, table string, join *JoinMeta, joinRegistry map[string]JoinMeta) error {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		// Recurse into embedded structs (e.g. gorm.Model) so their
		// promoted fields become schema entries. An unexported embedded
		// struct still promotes its exported fields, so this runs before
		// the export check. Named struct fields (time.Time,
		// gorm.DeletedAt, ...) are leaf columns, not recursed.
		if f.Anonymous {
			ft := derefType(f.Type)
			if ft.Kind() == reflect.Struct {
				if err := collectFields(ft, out, table, join, joinRegistry); err != nil {
					return err
				}
				continue
			}
		}

		if !f.IsExported() {
			continue
		}

		tag := f.Tag.Get(TagKey)
		if tag == "-" {
			continue
		}

		// A join tag turns a nested struct field into a joined table:
		// recurse into it with the join's table as the column qualifier
		// and the JoinMeta attached to every leaf.
		if jm, rest, ok, err := parseJoinTag(f, tag); err != nil {
			return err
		} else if ok {
			if rest != "" {
				return fmt.Errorf("FromModel: field %s: unknown tag part %q", f.Name, rest)
			}
			ft := derefType(f.Type)
			if ft.Kind() != reflect.Struct {
				return fmt.Errorf("FromModel: field %s: join tag requires a struct field, got %s", f.Name, ft.Kind())
			}
			if err := collectFields(ft, out, joinQualifier(jm.Table), jm, joinRegistry); err != nil {
				return err
			}
			continue
		}

		// Plain association structs/slices without a join tag have no
		// single column; skip them unless the type is database-scalar
		// (time.Time, sql.NullString, driver.Valuer, ...).
		if !isScalarType(f.Type) {
			continue
		}

		meta, name, err := buildFieldMeta(f, tag, table, join, joinRegistry)
		if err != nil {
			return err
		}
		if _, dup := out[name]; dup {
			return fmt.Errorf("FromModel: duplicate field name %q (field %s); use name: to disambiguate", name, f.Name)
		}
		out[name] = meta
	}
	return nil
}

// buildFieldMeta parses one leaf field's `sqlgen` tag and resolves its
// SQL column and query-facing field name. table, when non-empty,
// qualifies the resolved column ("id" → "table.id") unless a column:
// override is present, which is always used verbatim. join is attached
// to the FieldMeta so the Generator auto-applies it. joinRegistry
// resolves the joins: tag part (see below) to previously-declared joins.
func buildFieldMeta(f reflect.StructField, tag, table string, join *JoinMeta, joinRegistry map[string]JoinMeta) (FieldMeta, string, error) {
	baseColumn := resolveColumn(f)
	name := baseColumn
	nameSet := false
	columnOverride := ""
	operators := presetOperatorSet("all")
	searchable := isStringKind(f.Type)
	var extraJoins []JoinMeta

	for _, p := range splitTagParts(tag) {
		key, value, hasValue := p.key, p.value, p.hasValue

		switch key {
		case "joins":
			if !hasValue || value == "" {
				return FieldMeta{}, "", fmt.Errorf("FromModel: field %s: joins tag needs a list, e.g. joins:bond_details", f.Name)
			}
			for _, tableName := range strings.Split(value, ",") {
				tableName = strings.TrimSpace(tableName)
				if tableName == "" {
					continue
				}
				jm, ok := joinRegistry[tableName]
				if !ok {
					return FieldMeta{}, "", fmt.Errorf("FromModel: field %s: joins references undeclared join %q", f.Name, tableName)
				}
				extraJoins = append(extraJoins, jm)
			}
		case "filter":
			if !hasValue || value == "" {
				return FieldMeta{}, "", fmt.Errorf("FromModel: field %s: filter tag needs a list, e.g. filter:eq,contains", f.Name)
			}
			ops, err := parseOperatorList(value)
			if err != nil {
				return FieldMeta{}, "", fmt.Errorf("FromModel: field %s: %w", f.Name, err)
			}
			operators = ops
		case "search":
			searchable = true
		case "nosearch":
			searchable = false
		case "column":
			if !hasValue || value == "" {
				return FieldMeta{}, "", fmt.Errorf("FromModel: field %s: column tag needs a name, e.g. column:type_id", f.Name)
			}
			columnOverride = value
			if !nameSet {
				name = value
			}
		case "name":
			if !hasValue || value == "" {
				return FieldMeta{}, "", fmt.Errorf("FromModel: field %s: name tag needs a value, e.g. name:is_syariah", f.Name)
			}
			name = value
			nameSet = true
		default:
			return FieldMeta{}, "", fmt.Errorf("FromModel: field %s: unknown tag part %q", f.Name, p.raw)
		}
	}

	column := columnOverride
	if column == "" {
		column = baseColumn
		if table != "" {
			column = table + "." + column
		}
	}

	return FieldMeta{
		Column:     column,
		Join:       join,
		Joins:      extraJoins,
		Searchable: searchable,
		Operators:  operators,
	}, name, nil
}

// parseJoinTag detects and parses the join form of a `sqlgen` tag:
//
//	join:left;table:mutual_fund_details;on:mutual_fund_details.product_id = investment_products.id
//
// It returns (join, leftoverParts, isJoin, error). leftoverParts contains
// any tag parts other than join/table/on, which are invalid on a join
// field. When the tag has no join: part, isJoin is false and the other
// return values are zero.
func parseJoinTag(f reflect.StructField, tag string) (*JoinMeta, string, bool, error) {
	var (
		isJoin   bool
		jm       JoinMeta
		leftover []string
	)
	for _, p := range splitTagParts(tag) {
		switch p.key {
		case "join":
			isJoin = true
			switch strings.ToLower(p.value) {
			case "", "left":
				jm.Type = "LEFT"
			case "inner":
				jm.Type = "INNER"
			default:
				return nil, "", false, fmt.Errorf("FromModel: field %s: unknown join type %q (want left or inner)", f.Name, p.value)
			}
		case "table":
			jm.Table = p.value
		case "on":
			jm.On = p.value
		default:
			leftover = append(leftover, p.raw)
		}
	}
	if !isJoin {
		return nil, "", false, nil
	}
	if jm.Table == "" || jm.On == "" {
		return nil, "", false, fmt.Errorf("FromModel: field %s: join tag requires table: and on: parts", f.Name)
	}
	return &jm, strings.Join(leftover, ";"), true, nil
}

// joinQualifier returns the identifier used to qualify a joined table's
// columns: the alias when the table declaration carries one
// ("mutual_fund_details f" → "f"), otherwise the table name itself.
func joinQualifier(table string) string {
	parts := strings.Fields(table)
	if len(parts) == 0 {
		return table
	}
	return parts[len(parts)-1]
}

// parseOperatorList expands a comma-separated list of operator aliases
// and/or preset names into an operator whitelist map.
func parseOperatorList(list string) (map[model.Operator]bool, error) {
	ops := map[model.Operator]bool{}
	for _, item := range strings.Split(list, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if preset, ok := operatorPresets[item]; ok {
			for _, op := range preset {
				ops[op] = true
			}
			continue
		}
		op, ok := operatorAliases[item]
		if !ok {
			return nil, fmt.Errorf("unknown filter operator %q", item)
		}
		ops[op] = true
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("filter tag resolved to no operators")
	}
	return ops, nil
}

// presetOperatorSet returns a fresh operator map for the named preset.
func presetOperatorSet(name string) map[model.Operator]bool {
	ops := map[model.Operator]bool{}
	for _, op := range operatorPresets[name] {
		ops[op] = true
	}
	return ops
}

// resolveColumn determines the SQL column for a struct field, in priority
// order: `gorm:"column:..."` tag, then GORM's snake_case naming strategy
// applied to the Go field name (so ID → id, TypeID → type_id, matching
// what GORM itself would use). A `sqlgen` column: override, when present,
// is applied afterwards by buildFieldMeta.
func resolveColumn(f reflect.StructField) string {
	if gormTag := f.Tag.Get("gorm"); gormTag != "" {
		for _, part := range strings.Split(gormTag, ";") {
			key, value, hasValue := strings.Cut(strings.TrimSpace(part), ":")
			if hasValue && strings.EqualFold(strings.TrimSpace(key), "column") {
				if v := strings.TrimSpace(value); v != "" {
					return v
				}
			}
		}
	}
	return gormschema.NamingStrategy{}.ColumnName("", f.Name)
}

// isStringKind reports whether t (unwrapping pointers) is string-backed,
// which controls the default Searchable value for untagged fields: the
// global search clause uses LIKE/ILIKE and is only valid on text columns.
func isStringKind(t reflect.Type) bool {
	return derefType(t).Kind() == reflect.String
}

// derefType unwraps pointer types down to their element type.
func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// valuerType is the driver.Valuer interface, implemented by sql.Null*
// wrappers, gorm.DeletedAt, decimal types, and similar database-scalar
// struct types.
var valuerType = reflect.TypeOf((*driver.Valuer)(nil)).Elem()

// timeType is time.Time, special-cased because it does not implement
// driver.Valuer yet always maps to a single column.
var timeType = reflect.TypeOf(time.Time{})

// isScalarType reports whether t (unwrapping pointers) maps to a single
// database column: primitive kinds, []byte, time.Time, and struct types
// implementing driver.Valuer. Association structs, slices, and maps do
// not, and are skipped by collectFields unless declared with a join tag.
func isScalarType(t reflect.Type) bool {
	t = derefType(t)

	switch t.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return true
	case reflect.Slice:
		return t.Elem().Kind() == reflect.Uint8 // []byte
	case reflect.Struct:
		return t == timeType || t.Implements(valuerType) || reflect.PointerTo(t).Implements(valuerType)
	default:
		return false
	}
}
