# SQL Query Generator

A flexible, type-safe Go package for generating dynamic GORM queries from frontend request parameters.

## Features

✅ **14 Filter Operators** - Comprehensive filtering with type-safe operators  
✅ **Complex Filter Groups** - AND/OR logic with nested groups  
✅ **Global Search** - Search across multiple fields simultaneously  
✅ **Multi-field Sorting** - Sort by multiple columns with ASC/DESC  
✅ **Pagination** - Built-in limit/offset with defaults  
✅ **Field Projection** - SELECT specific columns  
✅ **Automatic Joins** - Auto-apply joins based on requested fields  
✅ **Eager Loading** - Preload relationships  
✅ **Soft Delete Support** - Include/exclude soft-deleted records  
✅ **DISTINCT Queries** - Remove duplicates  
✅ **Query Validation** - Configurable limits and security  
✅ **Case-Sensitive Search** - Database-agnostic search configuration  

## Installation

```bash
go get -u sql-generator
```

## Quick Start

```go
import (
    "sql-generator"
    "sql-generator/model"
    "gorm.io/gorm"
)

// 1. Define your schema metadata
generator := &sql_generator.Generator{
    Schema: &sql_generator.ModelMeta{
        Fields: map[string]sql_generator.FieldMeta{
            "name": {
                Column:     "name",
                Searchable: true,
                Operators: map[model.Operator]bool{
                    model.IsEqual:   true,
                    model.IsContain: true,
                },
            },
        },
    },
    DefaultFieldForSort: "id",
    CaseSensitiveSearch: false,
    MaxFiltersPerQuery:  50,
    MaxSortsPerQuery:    10,
}

// 2. Build your query from frontend request
query := model.Query{
    SelectParameter: model.SelectParameter{
        Filters: []model.Filter{
            {
                FieldName: "name",
                Operator:  model.IsContain,
                Value:     "john",
            },
        },
        PageDescriptor: model.Pagination{
            Page:     1,
            PageSize: 20,
        },
    },
}

// 3. Generate and apply scopes
scopes, err := generator.Scopes(query)
if err != nil {
    log.Fatal(err)
}

// 4. Execute query
var users []User
db.Scopes(scopes...).Find(&users)
```

## Supported Operators

### Comparison Operators
- `IS_EQUAL` - Exact match (`=`)
- `IS_NOT_EQUAL` - Not equal (`<>`)
- `IS_LESS_THAN` - Less than (`<`)
- `IS_MORE_THAN` - Greater than (`>`)
- `IS_LESS_THAN_OR_EQUAL` - Less than or equal (`<=`)
- `IS_MORE_THAN_OR_EQUAL` - Greater than or equal (`>=`)

### String Operators
- `IS_CONTAIN` - Contains substring (`LIKE %value%`)
- `IS_BEGIN_WITH` - Starts with (`LIKE value%`)
- `IS_END_WITH` - Ends with (`LIKE %value`)

### Range Operators
- `IS_BETWEEN` - Between two values (`BETWEEN x AND y`)
- `IS_IN` - In list (`IN (...)`)
- `IS_NOT_IN` - Not in list (`NOT IN (...)`)

### NULL Operators
- `IS_NULL` - Is NULL
- `IS_NOT_NULL` - Is NOT NULL

## Advanced Features

### Complex Filter Groups with AND/OR Logic

```go
query := model.Query{
    SelectParameter: model.SelectParameter{
        FilterGroups: []model.FilterGroup{
            {
                Condition: model.Or,
                Filters: []model.Filter{
                    {FieldName: "role", Operator: model.IsEqual, Value: "admin"},
                    {FieldName: "role", Operator: model.IsEqual, Value: "moderator"},
                },
            },
            {
                Condition: model.And,
                Filters: []model.Filter{
                    {FieldName: "status", Operator: model.IsEqual, Value: "active"},
                    {FieldName: "age", Operator: model.IsBetween, RangeValues: []any{18, 65}},
                },
            },
        },
    },
}
// SQL: ((role = 'admin' OR role = 'moderator')) AND ((status = 'active' AND age BETWEEN 18 AND 65))
```

### Nested Filter Groups

```go
filterGroup := model.FilterGroup{
    Condition: model.And,
    Filters: []model.Filter{
        {FieldName: "status", Operator: model.IsEqual, Value: "active"},
    },
    FilterGroups: []model.FilterGroup{
        {
            Condition: model.Or,
            Filters: []model.Filter{
                {FieldName: "role", Operator: model.IsEqual, Value: "admin"},
                {FieldName: "role", Operator: model.IsEqual, Value: "moderator"},
            },
        },
    },
}
// SQL: (status = 'active' AND (role = 'admin' OR role = 'moderator'))
```

### Global Search

Search across all searchable fields with one query parameter:

```go
query := model.Query{
    Search: "john", // Searches all fields marked as Searchable: true
}
```

### Field Projection

Select only specific columns:

```go
query := model.Query{
    SelectParameter: model.SelectParameter{
        Fields: []string{"id", "name", "email"}, // Only fetch these columns
    },
}
```

### Automatic Joins

Joins are automatically applied based on field metadata:

```go
Schema: &ModelMeta{
    Fields: map[string]FieldMeta{
        "user_name": {
            Column: "users.name",
            Join: &JoinMeta{
                Table: "users",
                On:    "users.id = orders.user_id",
                Type:  "LEFT", // or "INNER"
            },
        },
    },
}
```

When you filter/sort/select `user_name`, the join is automatically applied.

### Eager Loading (Preloads)

```go
query := model.Query{
    SelectParameter: model.SelectParameter{
        Preloads: []string{"User", "Comments", "Tags"},
    },
}
```

### Soft Delete Support

```go
query := model.Query{
    IncludeDeleted: true, // Include soft-deleted records (GORM Unscoped)
}
```

### DISTINCT Queries

```go
query := model.Query{
    Distinct: true, // Apply DISTINCT
}
```

### Multi-field Sorting

```go
query := model.Query{
    SelectParameter: model.SelectParameter{
        Sorts: []model.Sort{
            {FieldName: "status", SortDirection: model.Ascending},
            {FieldName: "created_at", SortDirection: model.Descending},
        },
    },
}
// SQL: ORDER BY status ASC, created_at DESC
```

## Configuration

### Generator Options

```go
generator := &sql_generator.Generator{
    // Required: Field metadata
    Schema: &sql_generator.ModelMeta{
        Fields: map[string]sql_generator.FieldMeta{ /* ... */ },
    },
    
    // Default sort field when no sorts specified
    DefaultFieldForSort: "created_at",
    
    // Case-sensitive search (false = ILIKE, true = LIKE)
    CaseSensitiveSearch: false,
    
    // Security limits (0 = unlimited)
    MaxFiltersPerQuery: 50,
    MaxSortsPerQuery:   10,
}
```

### Field Metadata

```go
FieldMeta{
    Column:     "database_column_name", // Required
    Searchable: true,                   // Include in global search
    
    // Whitelist allowed operators per field
    Operators: map[model.Operator]bool{
        model.IsEqual:    true,
        model.IsContain:  true,
        model.IsBetween:  true,
        // ... only allowed operators
    },
    
    // Optional: Join configuration
    Join: &JoinMeta{
        Table: "related_table",
        On:    "related_table.id = main_table.foreign_key",
        Type:  "LEFT", // or "INNER"
    },
}
```

## Security Features

1. **Field Whitelisting** - Only fields in schema can be queried
2. **Operator Whitelisting** - Each field specifies allowed operators
3. **Query Limits** - Configurable max filters and sorts
4. **Validation** - Input validation before query execution
5. **SQL Injection Protection** - Uses GORM parameterized queries

## Error Handling

```go
scopes, err := generator.Scopes(query)
if err != nil {
    // Possible errors:
    // - Unknown field
    // - Disallowed operator
    // - Too many filters/sorts
    // - Invalid range values (e.g., IS_BETWEEN without 2 values)
    log.Printf("Query validation failed: %v", err)
    return
}
```

## Frontend Integration Example

### JSON Request Format

```json
{
  "search": "john",
  "selectParameter": {
    "filters": [
      {
        "fieldName": "status",
        "operator": "IS_EQUAL",
        "value": "active"
      },
      {
        "fieldName": "age",
        "operator": "IS_BETWEEN",
        "rangeValues": [18, 65]
      }
    ],
    "filterGroups": [
      {
        "condition": "OR",
        "filters": [
          {"fieldName": "role", "operator": "IS_EQUAL", "value": "admin"},
          {"fieldName": "role", "operator": "IS_EQUAL", "value": "moderator"}
        ]
      }
    ],
    "sorts": [
      {"fieldName": "name", "sortDirection": "ASCENDING"}
    ],
    "pageDescriptor": {
      "page": 1,
      "pageSize": 20
    },
    "fields": ["id", "name", "email"],
    "preloads": ["Profile", "Posts"]
  },
  "includeDeleted": false,
  "distinct": false
}
```

### HTTP Handler Example

```go
func SearchUsers(c *gin.Context) {
    var query model.Query
    if err := c.ShouldBindJSON(&query); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    scopes, err := generator.Scopes(query)
    if err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    var users []User
    var total int64
    
    // Get total count
    db.Model(&User{}).Scopes(scopes[:len(scopes)-1]...).Count(&total)
    
    // Get paginated results
    db.Scopes(scopes...).Find(&users)
    
    c.JSON(200, gin.H{
        "data":  users,
        "total": total,
        "page":  query.SelectParameter.PageDescriptor.Page,
    })
}
```

## Best Practices

1. **Always set MaxFiltersPerQuery and MaxSortsPerQuery** to prevent abuse
2. **Whitelist operators per field** based on field type and use case
3. **Mark only necessary fields as Searchable** to optimize performance
4. **Use FilterGroups** instead of simple Filters for complex conditions
5. **Configure case sensitivity** based on your database (PostgreSQL: ILIKE, MySQL: COLLATE)
6. **Add indexes** on frequently filtered/sorted columns
7. **Test generated SQL** in development to ensure optimal performance

## Performance Tips

- Use field projection (`Fields`) to select only needed columns
- Limit pagination size (enforce max PageSize in your handler)
- Add database indexes for filtered/sorted fields
- Use preloads wisely (only load needed relationships)
- Consider caching for expensive queries

## Testing

Run tests:
```bash
go test -v ./...
```

See `generator_test.go` for comprehensive test examples.

## Examples

See `example/example.go` for 10+ real-world usage examples.

## License

MIT

## Contributing

Pull requests welcome! Please ensure tests pass before submitting.
