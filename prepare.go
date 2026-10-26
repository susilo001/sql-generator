package sql_generator

import (
	"fmt"
	"math"
	"strconv"

	"github.com/susilo001/sql-generator/model"
	"gorm.io/gorm"
)

// Prepare validates and normalizes a query without mutating the caller's slices.
// Direct Go zero pagination selects defaults; negative values fail in strict mode.
func (g *Generator) Prepare(query model.Query) (model.Query, error) {
	if err := g.validateQuery(query); err != nil {
		return model.Query{}, err
	}
	page := query.SelectParameter.PageDescriptor
	if g.StrictQueries && (page.Page < 0 || page.PageSize < 0) {
		return model.Query{}, fmt.Errorf("pagination must be positive")
	}
	if page.Page < 1 {
		page.Page = 1
	}
	if page.PageSize < 1 {
		page.PageSize = 20
	}
	if g.MaxPageSize > 0 && page.PageSize > g.MaxPageSize {
		if g.StrictQueries {
			return model.Query{}, fmt.Errorf("page size exceeds %d", g.MaxPageSize)
		}
		page.PageSize = g.MaxPageSize
	}
	maxOffset := int(^uint(0) >> 1)
	if g.MaxOffset > 0 {
		maxOffset = g.MaxOffset
	}
	if page.Page-1 > maxOffset/page.PageSize {
		return model.Query{}, fmt.Errorf("pagination offset exceeds %d", maxOffset)
	}
	query.SelectParameter.PageDescriptor = page
	sorts := append([]model.Sort(nil), query.SelectParameter.Sorts...)
	if g.StrictQueries {
		for _, sort := range sorts {
			if _, ok := g.Schema.Fields[sort.FieldName]; !ok || (g.Schema.AllowedSorts != nil && !g.Schema.AllowedSorts[sort.FieldName]) {
				return model.Query{}, fmt.Errorf("sort field not allowed: %s", sort.FieldName)
			}
			if sort.SortDirection != model.Ascending && sort.SortDirection != model.Descending {
				return model.Query{}, fmt.Errorf("invalid sort direction")
			}
		}
		for _, field := range query.SelectParameter.Fields {
			if _, ok := g.Schema.Fields[field]; !ok {
				return model.Query{}, fmt.Errorf("unknown selected field: %s", field)
			}
		}
	}
	if g.TieBreakerField != "" {
		if _, ok := g.Schema.Fields[g.TieBreakerField]; !ok {
			return model.Query{}, fmt.Errorf("unknown tie-breaker field")
		}
	}
	query.SelectParameter.Sorts = sorts
	filters, err := g.prepareFilters(query.SelectParameter.Filters)
	if err != nil {
		return model.Query{}, err
	}
	query.SelectParameter.Filters = filters
	groups, err := g.prepareGroups(query.SelectParameter.FilterGroups)
	if err != nil {
		return model.Query{}, err
	}
	query.SelectParameter.FilterGroups = groups
	return query, nil
}

func (g *Generator) prepareFilters(filters []model.Filter) ([]model.Filter, error) {
	prepared := append([]model.Filter(nil), filters...)
	for index := range prepared {
		filter := &prepared[index]
		field, ok := g.Schema.Fields[filter.FieldName]
		if !ok || !field.Operators[filter.Operator] {
			return nil, fmt.Errorf("filter field or operator not allowed: %s", filter.FieldName)
		}
		if filter.Operator == model.IsBetween && len(filter.RangeValues) != 2 {
			return nil, fmt.Errorf("IS_BETWEEN requires exactly 2 values")
		}
		if err := g.validateRangeValues(*filter); err != nil {
			return nil, err
		}
		if filter.Operator == model.IsNull || filter.Operator == model.IsNotNull {
			continue
		}
		var err error
		if filter.Operator == model.IsBetween || filter.Operator == model.IsIn || filter.Operator == model.IsNotIn {
			filter.RangeValues = append([]any(nil), filter.RangeValues...)
			for index, value := range filter.RangeValues {
				filter.RangeValues[index], err = prepareValue(value, field.ValueType)
				if err != nil {
					break
				}
			}
		} else {
			filter.Value, err = prepareValue(filter.Value, field.ValueType)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid value for %s: %w", filter.FieldName, err)
		}
	}
	return prepared, nil
}

func (g *Generator) prepareGroups(groups []model.FilterGroup) ([]model.FilterGroup, error) {
	prepared := append([]model.FilterGroup(nil), groups...)
	for index := range prepared {
		group := &prepared[index]
		if g.StrictQueries && group.Condition != model.And && group.Condition != model.Or {
			return nil, fmt.Errorf("invalid filter group condition")
		}
		var err error
		group.Filters, err = g.prepareFilters(group.Filters)
		if err != nil {
			return nil, err
		}
		group.FilterGroups, err = g.prepareGroups(group.FilterGroups)
		if err != nil {
			return nil, err
		}
	}
	return prepared, nil
}

func prepareValue(value any, kind string) (any, error) {
	switch kind {
	case "":
		return value, nil
	case "string":
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string")
		}
		return text, nil
	case "int":
		return strconv.Atoi(fmt.Sprint(value))
	case "float64":
		number, err := strconv.ParseFloat(fmt.Sprint(value), 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, fmt.Errorf("expected finite number")
		}
		return number, nil
	default:
		return nil, fmt.Errorf("unsupported value type: %s", kind)
	}
}

// CountScopes applies the same validation, filters, joins and search as Scopes,
// excluding ordering, projection, preloading and pagination.
func (g *Generator) CountScopes(query model.Query) ([]func(*gorm.DB) *gorm.DB, error) {
	prepared, err := g.Prepare(query)
	if err != nil {
		return nil, err
	}
	filters, err := g.filterScopes(prepared)
	if err != nil {
		return nil, err
	}
	groups, err := g.filterGroupScopes(prepared)
	if err != nil {
		return nil, err
	}
	return append(append(filters, groups...), g.joinScope(prepared), g.searchScope(prepared)), nil
}
