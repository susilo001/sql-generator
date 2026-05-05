package sql_generator

import (
	"fmt"
	"github.com/susilo001/sql-generator/model"
	"strings"

	"gorm.io/gorm"
)

type ModelMeta struct {
	Fields map[string]FieldMeta
}

type FieldMeta struct {
	Column     string
	Join       *JoinMeta
	Searchable bool
	Operators  map[model.Operator]bool
}

type JoinMeta struct {
	Table string
	On    string
	Type  string // LEFT / INNER
}

type Generator struct {
	Schema              *ModelMeta
	DefaultFieldForSort string
	CaseSensitiveSearch bool // If false, uses ILIKE/case-insensitive
	MaxFiltersPerQuery  int  // Limit filters to prevent abuse (0 = unlimited)
	MaxSortsPerQuery    int  // Limit sorts (0 = unlimited)
}

func (g *Generator) Scopes(q model.Query) ([]func(*gorm.DB) *gorm.DB, error) {
	var scopes []func(*gorm.DB) *gorm.DB

	// Apply query limits validation
	if err := g.validateQuery(q); err != nil {
		return nil, err
	}

	fs, err := g.filterScopes(q)
	if err != nil {
		return nil, err
	}
	scopes = append(scopes, fs...)

	// Handle filter groups
	fgs, err := g.filterGroupScopes(q)
	if err != nil {
		return nil, err
	}
	scopes = append(scopes, fgs...)

	scopes = append(scopes,
		g.joinScope(q),
		g.searchScope(q),
		g.sortScope(q),
		g.selectScope(q),
		g.preloadScope(q),
		g.distinctScope(q),
		g.softDeleteScope(q),
		g.paginationScope(q),
	)

	return scopes, nil
}

func (g *Generator) filterScopes(q model.Query) ([]func(*gorm.DB) *gorm.DB, error) {
	var scopes []func(*gorm.DB) *gorm.DB

	for _, f := range q.SelectParameter.Filters {
		meta, ok := g.Schema.Fields[f.FieldName]
		if !ok {
			return nil, fmt.Errorf("unknown field: %s", f.FieldName)
		}

		if !meta.Operators[f.Operator] {
			return nil, fmt.Errorf("operator not allowed: %s", f.Operator)
		}

		col := meta.Column
		op := f.Operator
		val := f.Value
		rng := f.RangeValues

		// Validate BEFORE creating scope
		if op == model.IsBetween && len(rng) != 2 {
			return nil, fmt.Errorf("IS_BETWEEN requires exactly 2 values")
		}

		scope := func(db *gorm.DB) *gorm.DB {
			switch op {

			case model.IsEqual:
				return db.Where(col+" = ?", val)

			case model.IsNotEqual:
				return db.Where(col+" <> ?", val)

			case model.IsLessThan:
				return db.Where(col+" < ?", val)

			case model.IsMoreThan:
				return db.Where(col+" > ?", val)

			case model.IsLessThanOrEqual:
				return db.Where(col+" <= ?", val)

			case model.IsMoreThanOrEqual:
				return db.Where(col+" >= ?", val)

			case model.IsContain:
				return db.Where(col+" LIKE ?", "%"+fmt.Sprint(val)+"%")

			case model.IsBeginWith:
				return db.Where(col+" LIKE ?", fmt.Sprint(val)+"%")

			case model.IsEndWith:
				return db.Where(col+" LIKE ?", "%"+fmt.Sprint(val))

			case model.IsBetween:
				return db.Where(col+" BETWEEN ? AND ?", rng[0], rng[1])

			case model.IsIn:
				return db.Where(col+" IN ?", rng)

			case model.IsNotIn:
				return db.Where(col+" NOT IN ?", rng)

			case model.IsNull:
				return db.Where(col + " IS NULL")

			case model.IsNotNull:
				return db.Where(col + " IS NOT NULL")

			default:
				// unreachable due to earlier validation
				return db
			}
		}

		scopes = append(scopes, scope)
	}

	return scopes, nil
}

func (g *Generator) searchScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if q.Search == "" {
			return db
		}

		term := "%" + q.Search + "%"

		var ors []string
		var args []any

		likeOp := "LIKE"
		if !g.CaseSensitiveSearch {
			likeOp = "ILIKE" // PostgreSQL case-insensitive
		}

		for _, f := range g.Schema.Fields {
			if f.Searchable {
				ors = append(ors, f.Column+" "+likeOp+" ?")
				args = append(args, term)
			}
		}

		if len(ors) > 0 {
			return db.Where("("+strings.Join(ors, " OR ")+")", args...)
		}

		return db
	}
}

func (g *Generator) sortScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(q.SelectParameter.Sorts) == 0 {
			return db.Order(g.DefaultFieldForSort + " ASC")
		}

		for _, s := range q.SelectParameter.Sorts {
			meta, ok := g.Schema.Fields[s.FieldName]
			if !ok {
				continue
			}

			dir := "ASC"
			if s.SortDirection == model.Descending {
				dir = "DESC"
			}

			db = db.Order(meta.Column + " " + dir)
		}

		return db
	}
}

func (g *Generator) paginationScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		p := q.SelectParameter.PageDescriptor.Page
		s := q.SelectParameter.PageDescriptor.PageSize

		if p < 1 {
			p = 1
		}
		if s < 1 {
			s = 20
		}

		return db.Limit(s).Offset((p - 1) * s)
	}
}

// validateQuery checks query limits and validates input
func (g *Generator) validateQuery(q model.Query) error {
	if g.MaxFiltersPerQuery > 0 {
		totalFilters := len(q.SelectParameter.Filters)
		for _, fg := range q.SelectParameter.FilterGroups {
			totalFilters += g.countFiltersInGroup(fg)
		}
		if totalFilters > g.MaxFiltersPerQuery {
			return fmt.Errorf("too many filters: %d (max: %d)", totalFilters, g.MaxFiltersPerQuery)
		}
	}

	if g.MaxSortsPerQuery > 0 && len(q.SelectParameter.Sorts) > g.MaxSortsPerQuery {
		return fmt.Errorf("too many sorts: %d (max: %d)", len(q.SelectParameter.Sorts), g.MaxSortsPerQuery)
	}

	return nil
}

func (g *Generator) countFiltersInGroup(fg model.FilterGroup) int {
	count := len(fg.Filters)
	for _, nested := range fg.FilterGroups {
		count += g.countFiltersInGroup(nested)
	}
	return count
}

// filterGroupScopes processes FilterGroups with nested AND/OR logic
func (g *Generator) filterGroupScopes(q model.Query) ([]func(*gorm.DB) *gorm.DB, error) {
	var scopes []func(*gorm.DB) *gorm.DB

	for _, fg := range q.SelectParameter.FilterGroups {
		scope, err := g.buildFilterGroupScope(fg)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}

	return scopes, nil
}

func (g *Generator) buildFilterGroupScope(fg model.FilterGroup) (func(*gorm.DB) *gorm.DB, error) {
	return func(db *gorm.DB) *gorm.DB {
		var conditions []string
		var args []any

		// Process individual filters
		for _, f := range fg.Filters {
			meta, ok := g.Schema.Fields[f.FieldName]
			if !ok {
				continue // Skip unknown fields
			}

			if !meta.Operators[f.Operator] {
				continue // Skip disallowed operators
			}

			condition, filterArgs := g.buildFilterCondition(meta.Column, f)
			if condition != "" {
				conditions = append(conditions, condition)
				args = append(args, filterArgs...)
			}
		}

		// Process nested filter groups
		for _, nested := range fg.FilterGroups {
			nestedScope, _ := g.buildFilterGroupScope(nested)
			// Execute nested scope on a temporary db to extract SQL
			// This is a simplified approach - in production you'd need more sophisticated SQL building
			db = nestedScope(db)
		}

		if len(conditions) == 0 {
			return db
		}

		// Join with AND or OR based on condition
		joiner := " AND "
		if fg.Condition == model.Or {
			joiner = " OR "
		}

		sql := "(" + strings.Join(conditions, joiner) + ")"
		return db.Where(sql, args...)
	}, nil
}

func (g *Generator) buildFilterCondition(col string, f model.Filter) (string, []any) {
	switch f.Operator {
	case model.IsEqual:
		return col + " = ?", []any{f.Value}
	case model.IsNotEqual:
		return col + " <> ?", []any{f.Value}
	case model.IsLessThan:
		return col + " < ?", []any{f.Value}
	case model.IsMoreThan:
		return col + " > ?", []any{f.Value}
	case model.IsLessThanOrEqual:
		return col + " <= ?", []any{f.Value}
	case model.IsMoreThanOrEqual:
		return col + " >= ?", []any{f.Value}
	case model.IsContain:
		return col + " LIKE ?", []any{"%" + fmt.Sprint(f.Value) + "%"}
	case model.IsBeginWith:
		return col + " LIKE ?", []any{fmt.Sprint(f.Value) + "%"}
	case model.IsEndWith:
		return col + " LIKE ?", []any{"%" + fmt.Sprint(f.Value)}
	case model.IsBetween:
		if len(f.RangeValues) == 2 {
			return col + " BETWEEN ? AND ?", []any{f.RangeValues[0], f.RangeValues[1]}
		}
	case model.IsIn:
		return col + " IN ?", []any{f.RangeValues}
	case model.IsNotIn:
		return col + " NOT IN ?", []any{f.RangeValues}
	case model.IsNull:
		return col + " IS NULL", nil
	case model.IsNotNull:
		return col + " IS NOT NULL", nil
	}
	return "", nil
}

// joinScope automatically applies joins for fields that require them
func (g *Generator) joinScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		appliedJoins := make(map[string]bool)

		// Collect all fields used in query
		allFields := make(map[string]bool)
		for _, f := range q.SelectParameter.Filters {
			allFields[f.FieldName] = true
		}
		for _, s := range q.SelectParameter.Sorts {
			allFields[s.FieldName] = true
		}
		for _, field := range q.SelectParameter.Fields {
			allFields[field] = true
		}

		// Apply necessary joins
		for fieldName := range allFields {
			if meta, ok := g.Schema.Fields[fieldName]; ok && meta.Join != nil {
				joinKey := meta.Join.Table + meta.Join.On
				if !appliedJoins[joinKey] {
					joinType := meta.Join.Type
					if joinType == "" {
						joinType = "LEFT"
					}
					db = db.Joins(joinType + " JOIN " + meta.Join.Table + " ON " + meta.Join.On)
					appliedJoins[joinKey] = true
				}
			}
		}

		return db
	}
}

// selectScope handles field projection (SELECT specific columns)
func (g *Generator) selectScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(q.SelectParameter.Fields) == 0 {
			return db
		}

		var columns []string
		for _, fieldName := range q.SelectParameter.Fields {
			if meta, ok := g.Schema.Fields[fieldName]; ok {
				columns = append(columns, meta.Column)
			}
		}

		if len(columns) > 0 {
			return db.Select(columns)
		}

		return db
	}
}

// preloadScope handles eager loading of relationships
func (g *Generator) preloadScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		for _, preload := range q.SelectParameter.Preloads {
			db = db.Preload(preload)
		}
		return db
	}
}

// distinctScope applies DISTINCT to the query
func (g *Generator) distinctScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if q.Distinct {
			return db.Distinct()
		}
		return db
	}
}

// softDeleteScope handles soft-deleted records
func (g *Generator) softDeleteScope(q model.Query) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if q.IncludeDeleted {
			return db.Unscoped() // Include soft-deleted records
		}
		return db
	}
}
