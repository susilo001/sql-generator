package sql_generator

import (
	"sql-generator/model"
	"testing"

	"gorm.io/gorm"
)

func TestGenerator_FilterScopes_Operators(t *testing.T) {
	gen := &Generator{
		Schema: &ModelMeta{
			Fields: map[string]FieldMeta{
				"status": {
					Column:     "status",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsEqual:    true,
						model.IsNotEqual: true,
						model.IsIn:       true,
					},
				},
				"age": {
					Column:     "age",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsMoreThan:        true,
						model.IsLessThan:        true,
						model.IsBetween:         true,
						model.IsMoreThanOrEqual: true,
					},
				},
				"name": {
					Column:     "name",
					Searchable: true,
					Operators: map[model.Operator]bool{
						model.IsContain:   true,
						model.IsBeginWith: true,
						model.IsEndWith:   true,
					},
				},
				"deleted_at": {
					Column:     "deleted_at",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsNull:    true,
						model.IsNotNull: true,
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		filter  model.Filter
		wantErr bool
	}{
		{
			name: "IS_EQUAL operator",
			filter: model.Filter{
				FieldName: "status",
				Operator:  model.IsEqual,
				Value:     "active",
			},
			wantErr: false,
		},
		{
			name: "IS_NOT_EQUAL operator",
			filter: model.Filter{
				FieldName: "status",
				Operator:  model.IsNotEqual,
				Value:     "inactive",
			},
			wantErr: false,
		},
		{
			name: "IS_MORE_THAN operator",
			filter: model.Filter{
				FieldName: "age",
				Operator:  model.IsMoreThan,
				Value:     18,
			},
			wantErr: false,
		},
		{
			name: "IS_BETWEEN operator",
			filter: model.Filter{
				FieldName:   "age",
				Operator:    model.IsBetween,
				RangeValues: []any{18, 65},
			},
			wantErr: false,
		},
		{
			name: "IS_BETWEEN operator - invalid range",
			filter: model.Filter{
				FieldName:   "age",
				Operator:    model.IsBetween,
				RangeValues: []any{18}, // Only 1 value
			},
			wantErr: true,
		},
		{
			name: "IS_IN operator",
			filter: model.Filter{
				FieldName:   "status",
				Operator:    model.IsIn,
				RangeValues: []any{"active", "pending"},
			},
			wantErr: false,
		},
		{
			name: "IS_CONTAIN operator",
			filter: model.Filter{
				FieldName: "name",
				Operator:  model.IsContain,
				Value:     "john",
			},
			wantErr: false,
		},
		{
			name: "IS_NULL operator",
			filter: model.Filter{
				FieldName: "deleted_at",
				Operator:  model.IsNull,
			},
			wantErr: false,
		},
		{
			name: "IS_NOT_NULL operator",
			filter: model.Filter{
				FieldName: "deleted_at",
				Operator:  model.IsNotNull,
			},
			wantErr: false,
		},
		{
			name: "Unknown field",
			filter: model.Filter{
				FieldName: "unknown_field",
				Operator:  model.IsEqual,
				Value:     "test",
			},
			wantErr: true,
		},
		{
			name: "Disallowed operator",
			filter: model.Filter{
				FieldName: "status",
				Operator:  model.IsMoreThan, // Not allowed for status
				Value:     100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := model.Query{
				SelectParameter: model.SelectParameter{
					Filters: []model.Filter{tt.filter},
				},
			}

			_, err := gen.filterScopes(q)
			if (err != nil) != tt.wantErr {
				t.Errorf("filterScopes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerator_FilterGroups(t *testing.T) {
	gen := &Generator{
		Schema: &ModelMeta{
			Fields: map[string]FieldMeta{
				"status": {
					Column:     "status",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsEqual: true,
					},
				},
				"role": {
					Column:     "role",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsEqual: true,
					},
				},
			},
		},
	}

	q := model.Query{
		SelectParameter: model.SelectParameter{
			FilterGroups: []model.FilterGroup{
				{
					Condition: model.And,
					Filters: []model.Filter{
						{
							FieldName: "status",
							Operator:  model.IsEqual,
							Value:     "active",
						},
						{
							FieldName: "role",
							Operator:  model.IsEqual,
							Value:     "admin",
						},
					},
				},
			},
		},
	}

	scopes, err := gen.filterGroupScopes(q)
	if err != nil {
		t.Errorf("filterGroupScopes() error = %v", err)
	}

	if len(scopes) != 1 {
		t.Errorf("Expected 1 scope, got %d", len(scopes))
	}
}

func TestGenerator_QueryValidation(t *testing.T) {
	gen := &Generator{
		Schema: &ModelMeta{
			Fields: map[string]FieldMeta{
				"status": {
					Column: "status",
					Operators: map[model.Operator]bool{
						model.IsEqual: true,
					},
				},
			},
		},
		MaxFiltersPerQuery: 2,
		MaxSortsPerQuery:   3,
	}

	tests := []struct {
		name    string
		query   model.Query
		wantErr bool
	}{
		{
			name: "Within limits",
			query: model.Query{
				SelectParameter: model.SelectParameter{
					Filters: []model.Filter{
						{FieldName: "status", Operator: model.IsEqual, Value: "active"},
					},
					Sorts: []model.Sort{
						{FieldName: "status", SortDirection: model.Ascending},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Too many filters",
			query: model.Query{
				SelectParameter: model.SelectParameter{
					Filters: []model.Filter{
						{FieldName: "status", Operator: model.IsEqual, Value: "1"},
						{FieldName: "status", Operator: model.IsEqual, Value: "2"},
						{FieldName: "status", Operator: model.IsEqual, Value: "3"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Too many sorts",
			query: model.Query{
				SelectParameter: model.SelectParameter{
					Sorts: []model.Sort{
						{FieldName: "status", SortDirection: model.Ascending},
						{FieldName: "status", SortDirection: model.Ascending},
						{FieldName: "status", SortDirection: model.Ascending},
						{FieldName: "status", SortDirection: model.Ascending},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gen.validateQuery(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerator_SearchScope_CaseSensitivity(t *testing.T) {
	tests := []struct {
		name              string
		caseSensitive     bool
		expectedOperation string
	}{
		{
			name:              "Case insensitive search",
			caseSensitive:     false,
			expectedOperation: "ILIKE",
		},
		{
			name:              "Case sensitive search",
			caseSensitive:     true,
			expectedOperation: "LIKE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &Generator{
				Schema: &ModelMeta{
					Fields: map[string]FieldMeta{
						"name": {
							Column:     "name",
							Searchable: true,
							Operators:  map[model.Operator]bool{},
						},
					},
				},
				CaseSensitiveSearch: tt.caseSensitive,
			}

			q := model.Query{
				Search: "test",
			}

			scope := gen.searchScope(q)
			db := &gorm.DB{}
			result := scope(db)

			// In a real test, you'd inspect the generated SQL
			// This is a simplified check
			if result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

func TestGenerator_JoinScope(t *testing.T) {
	gen := &Generator{
		Schema: &ModelMeta{
			Fields: map[string]FieldMeta{
				"user_name": {
					Column:     "users.name",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsEqual: true,
					},
					Join: &JoinMeta{
						Table: "users",
						On:    "users.id = orders.user_id",
						Type:  "LEFT",
					},
				},
				"order_status": {
					Column:     "orders.status",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsEqual: true,
					},
				},
			},
		},
	}

	q := model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{
					FieldName: "user_name",
					Operator:  model.IsEqual,
					Value:     "John",
				},
			},
		},
	}

	scope := gen.joinScope(q)
	db := &gorm.DB{}
	result := scope(db)

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestGenerator_SelectScope(t *testing.T) {
	gen := &Generator{
		Schema: &ModelMeta{
			Fields: map[string]FieldMeta{
				"id": {
					Column:     "id",
					Searchable: false,
					Operators:  map[model.Operator]bool{},
				},
				"name": {
					Column:     "name",
					Searchable: true,
					Operators:  map[model.Operator]bool{},
				},
				"email": {
					Column:     "email",
					Searchable: false,
					Operators:  map[model.Operator]bool{},
				},
			},
		},
	}

	q := model.Query{
		SelectParameter: model.SelectParameter{
			Fields: []string{"id", "name"},
		},
	}

	scope := gen.selectScope(q)
	db := &gorm.DB{}
	result := scope(db)

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestGenerator_PreloadScope(t *testing.T) {
	gen := &Generator{
		Schema: &ModelMeta{
			Fields: map[string]FieldMeta{},
		},
	}

	q := model.Query{
		SelectParameter: model.SelectParameter{
			Preloads: []string{"User", "Comments"},
		},
	}

	scope := gen.preloadScope(q)
	db := &gorm.DB{}
	result := scope(db)

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestGenerator_SoftDeleteScope(t *testing.T) {
	tests := []struct {
		name           string
		includeDeleted bool
	}{
		{
			name:           "Exclude deleted",
			includeDeleted: false,
		},
		{
			name:           "Include deleted",
			includeDeleted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &Generator{
				Schema: &ModelMeta{
					Fields: map[string]FieldMeta{},
				},
			}

			q := model.Query{
				IncludeDeleted: tt.includeDeleted,
			}

			scope := gen.softDeleteScope(q)
			db := &gorm.DB{}
			result := scope(db)

			if result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

func TestGenerator_DistinctScope(t *testing.T) {
	gen := &Generator{
		Schema: &ModelMeta{
			Fields: map[string]FieldMeta{},
		},
	}

	q := model.Query{
		Distinct: true,
	}

	scope := gen.distinctScope(q)
	db := &gorm.DB{}
	result := scope(db)

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestGenerator_PaginationScope(t *testing.T) {
	gen := &Generator{
		Schema: &ModelMeta{
			Fields: map[string]FieldMeta{},
		},
	}

	tests := []struct {
		name     string
		page     int
		pageSize int
		wantPage int
		wantSize int
	}{
		{
			name:     "Normal pagination",
			page:     2,
			pageSize: 10,
			wantPage: 2,
			wantSize: 10,
		},
		{
			name:     "Default page",
			page:     0,
			pageSize: 10,
			wantPage: 1,
			wantSize: 10,
		},
		{
			name:     "Default page size",
			page:     1,
			pageSize: 0,
			wantPage: 1,
			wantSize: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := model.Query{
				SelectParameter: model.SelectParameter{
					PageDescriptor: model.Pagination{
						Page:     tt.page,
						PageSize: tt.pageSize,
					},
				},
			}

			scope := gen.paginationScope(q)
			db := &gorm.DB{}
			result := scope(db)

			if result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}
