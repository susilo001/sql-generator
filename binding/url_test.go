package binding

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/susilo001/sql-generator/model"
)

func TestParseRawURL_RealExample(t *testing.T) {
	raw := "https://example.com/v2/products?search=capital&sort=type_id:asc&pageSize=10&page=2&filter=type_id:1:equals|data_status:Active:equals"

	q, err := ParseRawURL(raw, nil)
	if err != nil {
		t.Fatalf("ParseRawURL error: %v", err)
	}

	if q.Search != "capital" {
		t.Errorf("Search = %q, want %q", q.Search, "capital")
	}
	if q.SelectParameter.PageDescriptor.Page != 2 {
		t.Errorf("Page = %d, want 2", q.SelectParameter.PageDescriptor.Page)
	}
	if q.SelectParameter.PageDescriptor.PageSize != 10 {
		t.Errorf("PageSize = %d, want 10", q.SelectParameter.PageDescriptor.PageSize)
	}

	wantSorts := []model.Sort{
		{FieldName: "type_id", SortDirection: model.Ascending},
	}
	if !reflect.DeepEqual(q.SelectParameter.Sorts, wantSorts) {
		t.Errorf("Sorts = %+v, want %+v", q.SelectParameter.Sorts, wantSorts)
	}

	wantFilters := []model.Filter{
		{FieldName: "type_id", Operator: model.IsEqual, Value: "1"},
		{FieldName: "data_status", Operator: model.IsEqual, Value: "Active"},
	}
	if !reflect.DeepEqual(q.SelectParameter.Filters, wantFilters) {
		t.Errorf("Filters = %+v, want %+v", q.SelectParameter.Filters, wantFilters)
	}
}

func TestParseFilters_AllOperators(t *testing.T) {
	tests := []struct {
		alias string
		want  model.Operator
	}{
		{"equals", model.IsEqual},
		{"notequals", model.IsNotEqual},
		{"greaterthan", model.IsMoreThan},
		{"greaterthanorequal", model.IsMoreThanOrEqual},
		{"lessthan", model.IsLessThan},
		{"lessthanorequal", model.IsLessThanOrEqual},
		{"contains", model.IsContain},
		{"startswith", model.IsBeginWith},
		{"endswith", model.IsEndWith},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			raw := "https://x/y?filter=name:foo:" + tt.alias
			q, err := ParseRawURL(raw, nil)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if len(q.SelectParameter.Filters) != 1 {
				t.Fatalf("want 1 filter, got %d", len(q.SelectParameter.Filters))
			}
			f := q.SelectParameter.Filters[0]
			if f.Operator != tt.want {
				t.Errorf("Operator = %v, want %v", f.Operator, tt.want)
			}
			if f.Value != "foo" {
				t.Errorf("Value = %v, want %q", f.Value, "foo")
			}
		})
	}
}

func TestParseFilters_CaseInsensitiveAlias(t *testing.T) {
	raw := "https://x/y?filter=name:foo:EQUALS|age:10:GreaterThan"
	q, err := ParseRawURL(raw, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if q.SelectParameter.Filters[0].Operator != model.IsEqual {
		t.Errorf("EQUALS should map to IsEqual, got %v", q.SelectParameter.Filters[0].Operator)
	}
	if q.SelectParameter.Filters[1].Operator != model.IsMoreThan {
		t.Errorf("GreaterThan should map to IsMoreThan, got %v", q.SelectParameter.Filters[1].Operator)
	}
}

func TestParseFilters_IsInSplitsCommaValues(t *testing.T) {
	raw := "https://x/y?filter=status:active,pending,new:isin"
	q, err := ParseRawURL(raw, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	f := q.SelectParameter.Filters[0]
	if f.Operator != model.IsIn {
		t.Fatalf("Operator = %v, want IsIn", f.Operator)
	}
	want := []any{"active", "pending", "new"}
	if !reflect.DeepEqual(f.RangeValues, want) {
		t.Errorf("RangeValues = %+v, want %+v", f.RangeValues, want)
	}
	if f.Value != nil {
		t.Errorf("Value should be nil for IsIn, got %v", f.Value)
	}
}

func TestParseFilters_IsInSingleValue(t *testing.T) {
	raw := "https://x/y?filter=status:active:isin"
	q, err := ParseRawURL(raw, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	f := q.SelectParameter.Filters[0]
	if !reflect.DeepEqual(f.RangeValues, []any{"active"}) {
		t.Errorf("RangeValues = %+v, want [active]", f.RangeValues)
	}
}

func TestParseFilters_IsNotIn(t *testing.T) {
	raw := "https://x/y?filter=status:banned,archived:isnotin"
	q, err := ParseRawURL(raw, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	f := q.SelectParameter.Filters[0]
	if f.Operator != model.IsNotIn {
		t.Errorf("Operator = %v, want IsNotIn", f.Operator)
	}
	if !reflect.DeepEqual(f.RangeValues, []any{"banned", "archived"}) {
		t.Errorf("RangeValues = %+v", f.RangeValues)
	}
}

func TestParseFilters_UnknownOperatorErrors(t *testing.T) {
	raw := "https://x/y?filter=name:foo:betwixt"
	_, err := ParseRawURL(raw, nil)
	if err == nil {
		t.Fatal("expected error for unknown operator, got nil")
	}
}

func TestParseFilters_MalformedClauseErrors(t *testing.T) {
	raw := "https://x/y?filter=name:foo" // only 2 parts
	_, err := ParseRawURL(raw, nil)
	if err == nil {
		t.Fatal("expected error for malformed filter, got nil")
	}
}

func TestParseSorts_MultipleFields(t *testing.T) {
	raw := "https://x/y?sort=name:asc,created_at:desc"
	q, err := ParseRawURL(raw, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []model.Sort{
		{FieldName: "name", SortDirection: model.Ascending},
		{FieldName: "created_at", SortDirection: model.Descending},
	}
	if !reflect.DeepEqual(q.SelectParameter.Sorts, want) {
		t.Errorf("Sorts = %+v, want %+v", q.SelectParameter.Sorts, want)
	}
}

func TestParseSorts_BadDirectionErrors(t *testing.T) {
	raw := "https://x/y?sort=name:sideways"
	_, err := ParseRawURL(raw, nil)
	if err == nil {
		t.Fatal("expected error for bad sort direction, got nil")
	}
}

func TestParsePage_NonIntErrors(t *testing.T) {
	raw := "https://x/y?page=abc"
	_, err := ParseRawURL(raw, nil)
	if err == nil {
		t.Fatal("expected error for non-int page, got nil")
	}
}

func TestParseFieldsAndPreloads(t *testing.T) {
	raw := "https://x/y?fields=id,name,email&preloads=Profile,Orders"
	q, err := ParseRawURL(raw, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !reflect.DeepEqual(q.SelectParameter.Fields, []string{"id", "name", "email"}) {
		t.Errorf("Fields = %+v", q.SelectParameter.Fields)
	}
	if !reflect.DeepEqual(q.SelectParameter.Preloads, []string{"Profile", "Orders"}) {
		t.Errorf("Preloads = %+v", q.SelectParameter.Preloads)
	}
}

func TestParseRequest_UsesRequestURL(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/products?search=capital&page=3", nil)
	q, err := ParseRequest(req, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if q.Search != "capital" {
		t.Errorf("Search = %q", q.Search)
	}
	if q.SelectParameter.PageDescriptor.Page != 3 {
		t.Errorf("Page = %d", q.SelectParameter.PageDescriptor.Page)
	}
}

func TestParseValues_CustomOpAliasesReplaceDefaults(t *testing.T) {
	opts := &URLParseOptions{
		OpAliases: map[string]model.Operator{
			"eq": model.IsEqual,
		},
	}
	// "equals" is no longer registered — must fail
	raw := "https://x/y?filter=name:foo:equals"
	if _, err := ParseRawURL(raw, opts); err == nil {
		t.Fatal("expected error since default aliases are replaced")
	}
	// "eq" works
	raw = "https://x/y?filter=name:foo:eq"
	q, err := ParseRawURL(raw, opts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if q.SelectParameter.Filters[0].Operator != model.IsEqual {
		t.Errorf("custom eq alias failed")
	}
}

func TestParseValues_EmptyQueryReturnsZero(t *testing.T) {
	q, err := ParseRawURL("https://x/y", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if q.Search != "" || len(q.SelectParameter.Filters) != 0 || len(q.SelectParameter.Sorts) != 0 {
		t.Errorf("expected zero-value Query, got %+v", q)
	}
}
