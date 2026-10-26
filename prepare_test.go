package sql_generator

import (
	"reflect"
	"strings"
	"testing"

	"github.com/susilo001/sql-generator/binding"
	"github.com/susilo001/sql-generator/model"
)

func strictTestGenerator() *Generator {
	return &Generator{
		StrictQueries: true, PortableSearch: true, TieBreakerField: "id",
		MaxPageSize: 100, MaxOffset: 100000, MaxSortsPerQuery: 1,
		Schema: &ModelMeta{
			AllowedSorts: map[string]bool{"price": true}, AllowedPreloads: map[string]bool{},
			Fields: map[string]FieldMeta{
				"id":    {Column: "id"},
				"price": {Column: "price", ValueType: "float64", Operators: map[model.Operator]bool{model.IsMoreThanOrEqual: true}},
				"name":  {Column: "name", Searchable: true},
			},
		},
	}
}

func TestStrictScopesRejectInvalidQueries(t *testing.T) {
	for _, raw := range []string{
		"?sort=unknown:asc", "?sort=name:asc", "?fields=unknown", "?page=-1", "?pageSize=101",
		"?page=1002&pageSize=100", "?filter=price:NaN:greaterthanorequal",
		"?filter=price:Inf:greaterthanorequal", "?filter=price:no:greaterthanorequal", "?preloads=Category",
	} {
		t.Run(raw, func(t *testing.T) {
			query, err := binding.ParseRawURL("https://example.test/"+raw, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := strictTestGenerator().Scopes(query); err == nil {
				t.Fatal("invalid query accepted")
			}
			if _, err := strictTestGenerator().CountScopes(query); err == nil {
				t.Fatal("invalid count query accepted")
			}
		})
	}
}

func TestPrepareConvertsValuesWithoutMutatingQuery(t *testing.T) {
	query, err := binding.ParseRawURL("https://example.test/?filter=price:10:greaterthanorequal", nil)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := strictTestGenerator().Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.SelectParameter.Filters[0].Value != float64(10) || query.SelectParameter.Filters[0].Value != "10" {
		t.Fatalf("prepared=%+v original=%+v", prepared, query)
	}
	if prepared.SelectParameter.PageDescriptor != (model.Pagination{Page: 1, PageSize: 20}) {
		t.Fatalf("unexpected defaults: %+v", prepared)
	}
}

func TestStrictScopesKeepCountUnpaginatedAndSortStable(t *testing.T) {
	query, err := binding.ParseRawURL("https://example.test/?search=WIDGET&filter=price:10:greaterthanorequal&sort=price:desc&page=2&pageSize=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	generator := strictTestGenerator()
	sql, args := buildSQL(t, generator, query)
	if !strings.Contains(sql, "LOWER(name) LIKE") || !strings.Contains(sql, "price DESC,id ASC") || !strings.Contains(sql, "OFFSET") {
		t.Fatalf("unexpected list SQL: %s", sql)
	}
	scopes, err := generator.CountScopes(query)
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	count := newDryRunDB(t).Table("products").Scopes(scopes...).Count(&total)
	countSQL := count.Statement.SQL.String()
	if count.Error != nil || strings.Contains(countSQL, "ORDER BY") || strings.Contains(countSQL, "LIMIT") || strings.Contains(countSQL, "OFFSET") {
		t.Fatalf("count SQL=%s error=%v", countSQL, count.Error)
	}
	if !reflect.DeepEqual(count.Statement.Vars, args[:len(count.Statement.Vars)]) {
		t.Fatalf("count and list predicates differ: count=%v list=%v", count.Statement.Vars, args)
	}
}
