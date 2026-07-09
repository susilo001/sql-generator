package sql_generator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/susilo001/sql-generator/model"

	gormschema "gorm.io/gorm/schema"
)

// TagKey is the struct tag key read by FromModel, e.g.
//
//	Name string `sqlgen:"filter:eq,contains;search"`
const TagKey = "sqlgen"

// Options carries the Generator settings that cannot be derived from the
// model struct itself. It mirrors the tunable fields on Generator; see the
// corresponding Generator field documentation for the semantics of each.
// The zero value is valid: no default sort field, ILIKE search, and no
// limits.
type Options struct {
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
	"all": {
		model.IsEqual, model.IsNotEqual,
		model.IsLessThan, model.IsMoreThan,
		model.IsLessThanOrEqual, model.IsMoreThanOrEqual,
		model.IsContain, model.IsBeginWith, model.IsEndWith,
		model.IsBetween, model.IsIn, model.IsNotIn,
		model.IsNull, model.IsNotNull,
	},
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
// Embedded (anonymous) structs such as gorm.Model are recursed into;
// unexported fields are skipped. Named struct fields (time.Time,
// gorm.DeletedAt, sql.NullString, ...) are treated as leaf columns, not
// recursed. Fields backed by joins must still be declared in a
// hand-written ModelMeta — struct tags describe flat columns only.
//
// FromModel fails fast: a malformed tag (unknown operator alias, unknown
// tag part, empty column override) or two fields resolving to the same
// column name return an error rather than being silently skipped.
func FromModel(m any, opts ...Options) (*Generator, error) {
	t := reflect.TypeOf(m)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("FromModel: expected a struct or pointer to struct, got %T", m)
	}

	fields := map[string]FieldMeta{}
	if err := collectFields(t, fields); err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("FromModel: %s has no usable fields", t.Name())
	}

	var o Options
	if len(opts) > 0 {
		o = opts[0]
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

// collectFields walks the struct type t and adds one FieldMeta per usable
// field to out, recursing into embedded (anonymous) struct fields. It
// returns an error on the first malformed tag or duplicate column name.
func collectFields(t reflect.Type, out map[string]FieldMeta) error {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		// Recurse into embedded structs (e.g. gorm.Model) so their
		// promoted fields become schema entries. An unexported embedded
		// struct still promotes its exported fields, so this runs before
		// the export check. Named struct fields (time.Time,
		// gorm.DeletedAt, ...) are leaf columns, not recursed.
		if f.Anonymous {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				if err := collectFields(ft, out); err != nil {
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

		meta, name, err := buildFieldMeta(f, tag)
		if err != nil {
			return err
		}
		if _, dup := out[name]; dup {
			return fmt.Errorf("FromModel: duplicate column %q (field %s)", name, f.Name)
		}
		out[name] = meta
	}
	return nil
}

// buildFieldMeta parses one struct field's `sqlgen` tag and resolves its
// column name, returning the FieldMeta and the query-facing field name
// (which equals the resolved column).
func buildFieldMeta(f reflect.StructField, tag string) (FieldMeta, string, error) {
	column := resolveColumn(f)
	operators := presetOperatorSet("all")
	searchable := isStringKind(f.Type)

	for _, part := range strings.Split(tag, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, hasValue := strings.Cut(part, ":")
		key = strings.ToLower(strings.TrimSpace(key))

		switch key {
		case "filter":
			if !hasValue || strings.TrimSpace(value) == "" {
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
			value = strings.TrimSpace(value)
			if !hasValue || value == "" {
				return FieldMeta{}, "", fmt.Errorf("FromModel: field %s: column tag needs a name, e.g. column:type_id", f.Name)
			}
			column = value
		default:
			return FieldMeta{}, "", fmt.Errorf("FromModel: field %s: unknown tag part %q", f.Name, part)
		}
	}

	return FieldMeta{
		Column:     column,
		Searchable: searchable,
		Operators:  operators,
	}, column, nil
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
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.String
}
