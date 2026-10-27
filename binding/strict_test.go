package binding

import (
	"net/url"
	"testing"
)

func TestStrictPagination(t *testing.T) {
	for _, key := range []string{"page", "page_size"} {
		for _, value := range []string{"", "0", "-1", "1.5", "oops", "99999999999999999999"} {
			_, err := ParseValues(url.Values{key: {value}}, &URLParseOptions{StrictPagination: true, PageSizeKey: "page_size"})
			if err == nil {
				t.Fatalf("accepted %s=%q", key, value)
			}
		}
	}
}

func TestRepeatedFiltersAndPipes(t *testing.T) {
	query, err := ParseValues(url.Values{"filter": {"price:10:greaterthanorequal|price:20:lessthanorequal", "stock:0:equals"}}, nil)
	if err != nil || len(query.SelectParameter.Filters) != 3 {
		t.Fatalf("query=%+v error=%v", query, err)
	}
}

func TestStrictClausesRejectMalformedValues(t *testing.T) {
	for _, test := range []struct {
		name   string
		values url.Values
	}{
		{"empty filter", url.Values{"filter": {""}}},
		{"empty filter clause", url.Values{"filter": {"|"}}},
		{"trailing filter clause", url.Values{"filter": {"price:10:equals|"}}},
		{"empty repeated filter", url.Values{"filter": {"price:10:equals", ""}}},
		{"empty sort", url.Values{"sort": {""}}},
		{"empty sort clause", url.Values{"sort": {","}}},
		{"trailing sort clause", url.Values{"sort": {"price:asc,"}}},
		{"repeated sort", url.Values{"sort": {"price:asc", "name:desc"}}},
		{"empty fields", url.Values{"fields": {""}}},
		{"empty selected field", url.Values{"fields": {"name,"}}},
		{"empty preloads", url.Values{"preloads": {""}}},
		{"empty preload", url.Values{"preloads": {"Category,"}}},
		{"repeated preloads", url.Values{"preloads": {"Category", "Brand"}}},
		{"repeated page", url.Values{"page": {"1", "2"}}},
		{"repeated page size", url.Values{"pageSize": {"20", "40"}}},
		{"repeated search", url.Values{"search": {"first", "second"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseValues(test.values, &URLParseOptions{StrictClauses: true}); err == nil {
				t.Fatalf("accepted malformed values: %v", test.values)
			}
		})
	}
}

func TestStrictClausesAcceptRepeatedFilters(t *testing.T) {
	query, err := ParseValues(url.Values{"filter": {"price:10:greaterthanorequal|price:20:lessthanorequal", "stock:0:equals"}}, &URLParseOptions{StrictClauses: true})
	if err != nil || len(query.SelectParameter.Filters) != 3 {
		t.Fatalf("query=%+v error=%v", query, err)
	}
}

func TestStrictClausesHonorCustomKeysAndSeparators(t *testing.T) {
	options := &URLParseOptions{
		StrictClauses: true, FilterKey: "where", SortKey: "order", FieldsKey: "select",
		PreloadsKey: "include", PageSizeKey: "size", FilterSep: ";", SortSep: "|", ListSep: ";",
	}
	if _, err := ParseValues(url.Values{"where": {"price:10:equals;"}}, options); err == nil {
		t.Fatal("accepted trailing custom filter separator")
	}
	if _, err := ParseValues(url.Values{"order": {"price:asc|"}}, options); err == nil {
		t.Fatal("accepted trailing custom sort separator")
	}
	if _, err := ParseValues(url.Values{"include": {"Category;"}}, options); err == nil {
		t.Fatal("accepted trailing custom preload separator")
	}
	query, err := ParseValues(url.Values{"where": {"price:10:equals", "stock:0:equals"}, "size": {"5"}}, options)
	if err != nil || len(query.SelectParameter.Filters) != 2 || query.SelectParameter.PageDescriptor.PageSize != 5 {
		t.Fatalf("query=%+v error=%v", query, err)
	}
}

func TestDefaultClauseParsingRemainsPermissive(t *testing.T) {
	for _, values := range []url.Values{
		{"filter": {"|"}},
		{"sort": {","}},
		{"sort": {"price:asc", "name:desc"}},
		{"preloads": {"Category,"}},
	} {
		if _, err := ParseValues(values, nil); err != nil {
			t.Fatalf("default parser rejected %v: %v", values, err)
		}
	}
}
