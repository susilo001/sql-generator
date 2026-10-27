package model

import (
	"reflect"
	"testing"
)

func TestListQueryGeneratorQuery(t *testing.T) {
	filters := []Filter{{FieldName: "price", Operator: IsMoreThanOrEqual, Value: float64(10)}}
	sorts := []Sort{{FieldName: "price", SortDirection: Descending}}
	query := ListQuery{
		Page: 2, PageSize: 5, Search: "widget", Filters: filters, Sorts: sorts,
		Fields: []string{"name"}, Preloads: []string{"Category"},
	}.GeneratorQuery()
	params := query.SelectParameter
	if query.Search != "widget" || query.Filter != "" || query.Sort != "" || len(params.FilterGroups) != 0 || params.PageDescriptor != (Pagination{Page: 2, PageSize: 5}) {
		t.Fatalf("unexpected query: %+v", query)
	}
	if !reflect.DeepEqual(params.Filters, filters) || !reflect.DeepEqual(params.Sorts, sorts) || len(params.Fields) != 1 || len(params.Preloads) != 1 {
		t.Fatalf("list fields not preserved: %+v", params)
	}
}
