package binding

import (
	"net/url"
	"testing"

	"github.com/susilo001/sql-generator/model"
)

func TestParseListValues(t *testing.T) {
	parsed, err := ParseListValues(url.Values{
		"page": {"2"}, "page_size": {"5"}, "search": {"widget"},
		"filter": {"price:10:greaterthanorequal", "stock:0:equals"},
		"sort":   {"price:desc,name:asc"}, "fields": {"name,price"}, "preload": {"Category"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Page != 2 || parsed.PageSize != 5 || parsed.Search != "widget" || len(parsed.Filters) != 2 || len(parsed.Sorts) != 2 || len(parsed.Fields) != 2 || len(parsed.Preloads) != 1 {
		t.Fatalf("unexpected parsed list: %+v", parsed)
	}
	if parsed.Filters[0].FieldName != "price" || parsed.Filters[0].Operator != model.IsMoreThanOrEqual || parsed.Filters[0].Value != "10" || parsed.Sorts[0].FieldName != "price" || parsed.Sorts[0].SortDirection != model.Descending {
		t.Fatalf("unexpected filter or sort: %+v", parsed)
	}
	if parsed.GeneratorQuery().SelectParameter.Filters[0].Operator != model.IsMoreThanOrEqual {
		t.Fatal("generator conversion changed filter")
	}
}

func TestParseListValuesRejectsMalformedQueries(t *testing.T) {
	for _, raw := range []string{
		"page=", "page=0", "page_size=", "page_size=-1", "page=1&page=2",
		"filter=", "filter=%7C", "filter=price:10:equals%7C", "filter=price:10:equals&filter=",
		"sort=", "sort=price:asc%2C", "sort=price:asc&sort=name:desc",
		"fields=", "preload=", "preload=Category%2C", "search=first&search=second",
	} {
		t.Run(raw, func(t *testing.T) {
			values, err := url.ParseQuery(raw)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseListValues(values); err == nil {
				t.Fatal("malformed list accepted")
			}
		})
	}
}

func TestParseListValuesKeepsAbsentPaginationUnset(t *testing.T) {
	parsed, err := ParseListValues(url.Values{})
	if err != nil || parsed.Page != 0 || parsed.PageSize != 0 {
		t.Fatalf("list=%+v error=%v", parsed, err)
	}
}
