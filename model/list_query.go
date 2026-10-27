package model

// ListQuery is a flat list request without Query's legacy raw fields or filter groups.
// URL parsing can populate it, while services may also construct it directly.
type ListQuery struct {
	Page     int
	PageSize int
	Search   string
	Filters  []Filter
	Sorts    []Sort
	Fields   []string
	Preloads []string
}

// GeneratorQuery converts the flat list contract to the generator's query model.
func (query ListQuery) GeneratorQuery() Query {
	return Query{
		Search: query.Search,
		SelectParameter: SelectParameter{
			Filters:        query.Filters,
			Sorts:          query.Sorts,
			PageDescriptor: Pagination{Page: query.Page, PageSize: query.PageSize},
			Fields:         query.Fields,
			Preloads:       query.Preloads,
		},
	}
}
