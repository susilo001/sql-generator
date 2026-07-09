package sql_generator

// This file measures, rather than assumes, the two properties users care
// most about:
//
//  1. Security — classic SQL-injection payloads placed in filter values,
//     search terms, and IN-lists must never appear in the generated SQL
//     text; they must only ever surface as parameterized bind variables.
//  2. Performance — how long Scopes() takes to compile queries of varying
//     complexity, and how long a full dry-run SQL build takes end to end.
//
// The SQL-rendering tests use GORM's DryRun mode with the DummyDialector
// from gorm.io/gorm/utils/tests, so they run without any database.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/susilo001/sql-generator/binding"
	"github.com/susilo001/sql-generator/model"

	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

// product mirrors the shape of a typical target table for dry-run builds.
type product struct {
	ID         int
	TypeID     int
	Name       string
	DataStatus string
}

func newSecuritySchema() *Generator {
	allOps := map[model.Operator]bool{
		model.IsEqual: true, model.IsNotEqual: true,
		model.IsLessThan: true, model.IsMoreThan: true,
		model.IsLessThanOrEqual: true, model.IsMoreThanOrEqual: true,
		model.IsContain: true, model.IsBeginWith: true, model.IsEndWith: true,
		model.IsBetween: true, model.IsIn: true, model.IsNotIn: true,
		model.IsNull: true, model.IsNotNull: true,
	}
	return &Generator{
		Schema: &ModelMeta{
			Fields: map[string]FieldMeta{
				"type_id":     {Column: "type_id", Operators: allOps},
				"data_status": {Column: "data_status", Operators: allOps},
				"name":        {Column: "name", Searchable: true, Operators: allOps},
			},
		},
		DefaultFieldForSort: "id",
		CaseSensitiveSearch: true, // plain LIKE so DummyDialector renders it
	}
}

// newDryRunDB returns a *gorm.DB that renders SQL without executing it.
func newDryRunDB(t testing.TB) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open dry-run db: %v", err)
	}
	return db
}

// buildSQL applies q through Scopes onto a dry-run Find and returns the
// rendered SQL text plus the bind variables.
func buildSQL(t testing.TB, gen *Generator, q model.Query) (string, []any) {
	t.Helper()
	scopes, err := gen.Scopes(q)
	if err != nil {
		t.Fatalf("Scopes: %v", err)
	}
	var out []product
	tx := newDryRunDB(t).Table("products").Scopes(scopes...).Find(&out)
	if tx.Error != nil {
		t.Fatalf("dry-run find: %v", tx.Error)
	}
	return tx.Statement.SQL.String(), tx.Statement.Vars
}

// injectionPayloads are representative attack strings. If any of these ever
// appears verbatim inside the SQL text (rather than in Vars), the
// parameterization guarantee is broken.
var injectionPayloads = []string{
	"'; DROP TABLE products; --",
	"1 OR 1=1",
	"1; DELETE FROM products",
	"' UNION SELECT password FROM users --",
	"\" OR \"\"=\"",
	"$1); DROP TABLE products; --",
}

// assertPayloadParameterized fails if payload leaked into the SQL text or
// is missing from the bind variables.
func assertPayloadParameterized(t *testing.T, sql string, vars []any, payload string) {
	t.Helper()
	if strings.Contains(sql, payload) {
		t.Fatalf("injection payload leaked into SQL text:\nsql: %s\npayload: %s", sql, payload)
	}
	for _, v := range vars {
		if s, ok := v.(string); ok && strings.Contains(s, payload) {
			return // payload safely confined to a bind variable
		}
	}
	t.Fatalf("payload not found in bind vars (unexpected): %q\nvars: %v", payload, vars)
}

func TestSecurity_FilterValueInjection(t *testing.T) {
	gen := newSecuritySchema()
	for _, payload := range injectionPayloads {
		t.Run(payload, func(t *testing.T) {
			q := model.Query{}
			q.SelectParameter.Filters = []model.Filter{
				{FieldName: "data_status", Operator: model.IsEqual, Value: payload},
			}
			sql, vars := buildSQL(t, gen, q)
			assertPayloadParameterized(t, sql, vars, payload)
		})
	}
}

func TestSecurity_SearchTermInjection(t *testing.T) {
	gen := newSecuritySchema()
	for _, payload := range injectionPayloads {
		t.Run(payload, func(t *testing.T) {
			q := model.Query{Search: payload}
			sql, vars := buildSQL(t, gen, q)
			// Search wraps the term in %...%, so check the raw payload.
			if strings.Contains(sql, payload) {
				t.Fatalf("search payload leaked into SQL text:\nsql: %s", sql)
			}
			found := false
			for _, v := range vars {
				if s, ok := v.(string); ok && strings.Contains(s, payload) {
					found = true
				}
			}
			if !found {
				t.Fatalf("search payload missing from bind vars: %q", payload)
			}
		})
	}
}

func TestSecurity_InListInjection(t *testing.T) {
	gen := newSecuritySchema()
	q := model.Query{}
	q.SelectParameter.Filters = []model.Filter{
		{
			FieldName:   "type_id",
			Operator:    model.IsIn,
			RangeValues: []any{"1", "2'; DROP TABLE products; --"},
		},
	}
	sql, vars := buildSQL(t, gen, q)
	if strings.Contains(sql, "DROP TABLE") {
		t.Fatalf("IN-list payload leaked into SQL text:\nsql: %s", sql)
	}
	if len(vars) == 0 {
		t.Fatal("expected IN-list values as bind vars, got none")
	}
}

func TestSecurity_LikeWildcardsNotEscaped(t *testing.T) {
	// Documents (does not fail on) a known behavior: % and _ in IsContain /
	// IsBeginWith / IsEndWith values are treated as LIKE wildcards, not
	// literals. A value of "%" matches every row. This is a filter-bypass /
	// full-table-scan concern, not an injection concern.
	gen := newSecuritySchema()
	q := model.Query{}
	q.SelectParameter.Filters = []model.Filter{
		{FieldName: "name", Operator: model.IsContain, Value: "%"},
	}
	_, vars := buildSQL(t, gen, q)
	if len(vars) == 0 || vars[0] != "%%%" {
		t.Logf("note: LIKE value rendered as %v — wildcards pass through unescaped", vars)
	}
}

func TestSecurity_FieldNameInjectionRejected(t *testing.T) {
	// A hostile field name must be rejected by the whitelist, never
	// concatenated into SQL.
	gen := newSecuritySchema()
	q := model.Query{}
	q.SelectParameter.Filters = []model.Filter{
		{FieldName: "1=1; DROP TABLE products; --", Operator: model.IsEqual, Value: "x"},
	}
	if _, err := gen.Scopes(q); err == nil {
		t.Fatal("expected unknown-field error for hostile field name, got nil")
	}
}

func TestSecurity_SortFieldInjectionSkipped(t *testing.T) {
	// Hostile sort field names are silently skipped (documented behavior);
	// the resulting SQL must not contain the hostile string.
	gen := newSecuritySchema()
	q := model.Query{}
	q.SelectParameter.Sorts = []model.Sort{
		{FieldName: "id; DROP TABLE products", SortDirection: model.Ascending},
	}
	sql, _ := buildSQL(t, gen, q)
	if strings.Contains(sql, "DROP TABLE") {
		t.Fatalf("hostile sort field leaked into SQL: %s", sql)
	}
}

func TestSecurity_URLBindingInjectionEndToEnd(t *testing.T) {
	// Full pipeline: hostile URL → binding → Scopes → dry-run SQL.
	gen := newSecuritySchema()
	raw := "/v2/products?search=" +
		"%27%3B%20DROP%20TABLE%20products%3B%20--" + // '; DROP TABLE products; --
		"&filter=data_status:1%20OR%201%3D1:equals"
	q, err := binding.ParseRawURL(raw, nil)
	if err != nil {
		t.Fatalf("ParseRawURL: %v", err)
	}
	sql, _ := buildSQL(t, gen, q)
	for _, needle := range []string{"DROP TABLE", "OR 1=1"} {
		if strings.Contains(sql, needle) {
			t.Fatalf("URL payload %q leaked into SQL: %s", needle, sql)
		}
	}
}

func TestSecurity_DeepNestingIsUnbounded(t *testing.T) {
	// Measures the known gap: FilterGroup nesting depth is not limited.
	// Depth 10 000 compiles without error today. This test exists so that
	// if a MaxFilterGroupDepth limit is added later, it will fail and force
	// an intentional update.
	gen := newSecuritySchema()
	leaf := model.FilterGroup{
		Condition: model.And,
		Filters: []model.Filter{
			{FieldName: "type_id", Operator: model.IsEqual, Value: 1},
		},
	}
	group := leaf
	for i := 0; i < 10_000; i++ {
		group = model.FilterGroup{Condition: model.Or, FilterGroups: []model.FilterGroup{group}}
	}
	q := model.Query{}
	q.SelectParameter.FilterGroups = []model.FilterGroup{group}
	if _, err := gen.Scopes(q); err != nil {
		t.Fatalf("deep nesting unexpectedly rejected: %v", err)
	}
	t.Log("confirmed: 10000-deep FilterGroup nesting is accepted (no depth limit exists)")
}

// --- Benchmarks -----------------------------------------------------------

func benchQuerySimple() model.Query {
	q := model.Query{}
	q.SelectParameter.Filters = []model.Filter{
		{FieldName: "type_id", Operator: model.IsEqual, Value: 1},
	}
	q.SelectParameter.PageDescriptor = model.Pagination{Page: 2, PageSize: 10}
	return q
}

func benchQueryComplex() model.Query {
	q := model.Query{Search: "capital"}
	for i := 0; i < 10; i++ {
		q.SelectParameter.Filters = append(q.SelectParameter.Filters, model.Filter{
			FieldName: "type_id", Operator: model.IsMoreThan, Value: i,
		})
	}
	q.SelectParameter.FilterGroups = []model.FilterGroup{{
		Condition: model.Or,
		Filters: []model.Filter{
			{FieldName: "data_status", Operator: model.IsEqual, Value: "Active"},
			{FieldName: "data_status", Operator: model.IsEqual, Value: "Draft"},
		},
		FilterGroups: []model.FilterGroup{{
			Condition: model.And,
			Filters: []model.Filter{
				{FieldName: "type_id", Operator: model.IsBetween, RangeValues: []any{1, 9}},
				{FieldName: "name", Operator: model.IsContain, Value: "cap"},
			},
		}},
	}}
	q.SelectParameter.Sorts = []model.Sort{
		{FieldName: "type_id", SortDirection: model.Ascending},
		{FieldName: "name", SortDirection: model.Descending},
	}
	q.SelectParameter.Fields = []string{"type_id", "name", "data_status"}
	q.SelectParameter.PageDescriptor = model.Pagination{Page: 2, PageSize: 10}
	return q
}

// BenchmarkScopes_Simple measures scope compilation alone (no SQL render)
// for a minimal one-filter query.
func BenchmarkScopes_Simple(b *testing.B) {
	gen := newSecuritySchema()
	q := benchQuerySimple()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := gen.Scopes(q); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkScopes_Complex measures scope compilation for a realistic heavy
// query: search + 10 filters + nested OR/AND groups + 2 sorts + projection.
func BenchmarkScopes_Complex(b *testing.B) {
	gen := newSecuritySchema()
	q := benchQueryComplex()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := gen.Scopes(q); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEndToEnd_DryRunSQL measures the full pipeline cost per request:
// Scopes compilation plus GORM statement building of the final SQL string.
// This is the realistic per-request CPU cost, excluding only network/DB time.
func BenchmarkEndToEnd_DryRunSQL(b *testing.B) {
	gen := newSecuritySchema()
	q := benchQueryComplex()
	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scopes, err := gen.Scopes(q)
		if err != nil {
			b.Fatal(err)
		}
		var out []product
		tx := db.Session(&gorm.Session{NewDB: true}).Table("products").Scopes(scopes...).Find(&out)
		if tx.Error != nil {
			b.Fatal(tx.Error)
		}
	}
}

// BenchmarkBinding_ParseRawURL measures URL query-string parsing for the
// real-world example URL.
func BenchmarkBinding_ParseRawURL(b *testing.B) {
	raw := "/v2/products?search=capital&sort=type_id:asc&pageSize=10&page=2&filter=type_id:1:equals|data_status:Active:equals"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := binding.ParseRawURL(raw, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkScopes_DeepNesting measures compile cost growth with FilterGroup
// nesting depth, quantifying the unbounded-recursion exposure.
func BenchmarkScopes_DeepNesting(b *testing.B) {
	gen := newSecuritySchema()
	for _, depth := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("depth-%d", depth), func(b *testing.B) {
			group := model.FilterGroup{
				Condition: model.And,
				Filters: []model.Filter{
					{FieldName: "type_id", Operator: model.IsEqual, Value: 1},
				},
			}
			for i := 0; i < depth; i++ {
				group = model.FilterGroup{Condition: model.Or, FilterGroups: []model.FilterGroup{group}}
			}
			q := model.Query{}
			q.SelectParameter.FilterGroups = []model.FilterGroup{group}
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := gen.Scopes(q); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
