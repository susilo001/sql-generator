package binding

import (
	"net/url"

	"github.com/susilo001/sql-generator/model"
)

// ParseListValues parses the shared list URL convention with strict validation.
// It uses page_size and preload keys and permits repeated filter parameters.
func ParseListValues(values url.Values) (model.ListQuery, error) {
	parsed, err := ParseValues(values, &URLParseOptions{
		PageSizeKey: "page_size", PreloadsKey: "preload", StrictPagination: true, StrictClauses: true,
	})
	if err != nil {
		return model.ListQuery{}, err
	}
	params := parsed.SelectParameter
	return model.ListQuery{
		Page: params.PageDescriptor.Page, PageSize: params.PageDescriptor.PageSize,
		Search: parsed.Search, Filters: params.Filters, Sorts: params.Sorts,
		Fields: params.Fields, Preloads: params.Preloads,
	}, nil
}
