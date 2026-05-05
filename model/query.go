package model

type Query struct {
	Search          string          `json:"search"`
	Filter          string          `json:"filter"`
	Sort            string          `json:"sort"`
	SelectParameter SelectParameter `json:"select_parameter"`
	IncludeDeleted  bool            `json:"include_deleted"`
	Distinct        bool            `json:"distinct"`
}

type SelectParameter struct {
	Filters        []Filter    `json:"filters"`
	FilterGroups   []FilterGroup `json:"filter_groups"`
	Sorts          []Sort      `json:"sorts"`
	PageDescriptor Pagination  `json:"page_descriptor"`
	Fields         []string    `json:"fields"`
	Preloads       []string    `json:"preloads"`
}
