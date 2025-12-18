# Changelog

All notable changes and improvements to the SQL Query Generator.

## [2.0.0] - 2025-12-18

### Added

#### Core Features
- **IS_NULL and IS_NOT_NULL operators** - Support for NULL checking in filters
- **Complex FilterGroup support** - AND/OR logic with nested groups for advanced queries
- **Field projection** - SELECT specific columns via `Fields []string`
- **Automatic join resolution** - Joins applied based on field metadata and query requirements
- **Eager loading (Preloads)** - Load relationships with `Preloads []string`
- **Soft delete support** - `IncludeDeleted` flag to include soft-deleted records
- **DISTINCT queries** - `Distinct bool` flag for removing duplicates
- **Case-sensitive search configuration** - `CaseSensitiveSearch` option for database-agnostic search

#### Security & Validation
- **Query validation** - Input validation before query execution
- **Configurable query limits**:
  - `MaxFiltersPerQuery` - Limit total filters per query
  - `MaxSortsPerQuery` - Limit total sorts per query
- **Filter counting in groups** - Recursive counting for nested filter groups

#### Generator Configuration
- `CaseSensitiveSearch bool` - Toggle between LIKE and ILIKE
- `MaxFiltersPerQuery int` - Security limit for filters (0 = unlimited)
- `MaxSortsPerQuery int` - Security limit for sorts (0 = unlimited)

#### Model Enhancements
- **SelectParameter** now includes:
  - `FilterGroups []FilterGroup` - Complex filter conditions
  - `Fields []string` - Field projection
  - `Preloads []string` - Eager loading relationships
- **Query** model additions:
  - `IncludeDeleted bool` - Include soft-deleted records
  - `Distinct bool` - Apply DISTINCT

#### FilterGroup Enhancements
- `FilterGroups []FilterGroup` - Support for nested groups (recursive)
- Unlimited nesting depth for complex query logic

### Changed

- **Search scope** - Now respects `CaseSensitiveSearch` configuration
- **Scopes method** - Added validation, filter groups, joins, select, preload, distinct, and soft delete scopes
- **Filter validation** - More comprehensive validation before scope generation
- **Documentation** - Comprehensive README with all features and examples

### New Methods

#### Generator Methods
- `validateQuery(q model.Query) error` - Validates query limits and structure
- `countFiltersInGroup(fg model.FilterGroup) int` - Recursive filter counting
- `filterGroupScopes(q model.Query) ([]func(*gorm.DB) *gorm.DB, error)` - Process filter groups
- `buildFilterGroupScope(fg model.FilterGroup) (func(*gorm.DB) *gorm.DB, error)` - Build scope for single group
- `buildFilterCondition(col string, f model.Filter) (string, []any)` - Extract filter condition building
- `joinScope(q model.Query) func(*gorm.DB) *gorm.DB` - Auto-apply joins
- `selectScope(q model.Query) func(*gorm.DB) *gorm.DB` - Field projection
- `preloadScope(q model.Query) func(*gorm.DB) *gorm.DB` - Eager loading
- `distinctScope(q model.Query) func(*gorm.DB) *gorm.DB` - DISTINCT queries
- `softDeleteScope(q model.Query) func(*gorm.DB) *gorm.DB` - Soft delete handling

### Operators

Now supporting 14 total operators:

**Comparison:**
- IS_EQUAL
- IS_NOT_EQUAL
- IS_LESS_THAN
- IS_MORE_THAN
- IS_LESS_THAN_OR_EQUAL
- IS_MORE_THAN_OR_EQUAL

**String:**
- IS_CONTAIN
- IS_BEGIN_WITH
- IS_END_WITH

**Range:**
- IS_BETWEEN
- IS_IN
- IS_NOT_IN

**NULL:**
- IS_NULL (NEW)
- IS_NOT_NULL (NEW)

### Examples & Documentation

- **example/example.go** - 10 comprehensive usage examples
- **README.md** - Complete documentation with:
  - Feature overview
  - Quick start guide
  - All operators documented
  - Advanced features guide
  - Configuration options
  - Security features
  - Frontend integration examples
  - Best practices
  - Performance tips
- **generator_test.go** - Comprehensive test suite covering:
  - All 14 operators
  - Filter groups
  - Query validation
  - Case sensitivity
  - Joins
  - Field projection
  - Preloads
  - Soft deletes
  - DISTINCT queries
  - Pagination

### Migration Guide from 1.x

#### Breaking Changes
- `SelectParameter.Filters` is now supplemented (not replaced) by `SelectParameter.FilterGroups`
- No breaking changes to existing code - all 1.x features remain compatible

#### New Features to Adopt
```go
// 1. Add query limits for security
generator.MaxFiltersPerQuery = 50
generator.MaxSortsPerQuery = 10

// 2. Configure search case sensitivity
generator.CaseSensitiveSearch = false // for ILIKE

// 3. Use FilterGroups for complex queries
query.SelectParameter.FilterGroups = []model.FilterGroup{
    {
        Condition: model.Or,
        Filters: []model.Filter{...},
    },
}

// 4. Use field projection to optimize queries
query.SelectParameter.Fields = []string{"id", "name"}

// 5. Preload relationships
query.SelectParameter.Preloads = []string{"User", "Comments"}

// 6. Handle soft deletes
query.IncludeDeleted = true

// 7. Use DISTINCT when needed
query.Distinct = true
```

## [1.0.0] - Initial Release

### Features
- Basic filter operators (12 operators)
- Simple filters with AND logic
- Global search across fields
- Multi-field sorting
- Pagination
- Field metadata configuration
- Operator whitelisting per field
- Searchable field marking

---

## Upgrade Path

**From 1.x to 2.0:**
1. No code changes required - fully backward compatible
2. Add new configuration options as needed
3. Adopt FilterGroups for complex queries
4. Enable query validation for security
5. Use new features (joins, preloads, field projection) incrementally

**Benefits of upgrading:**
- 2 new NULL operators
- Complex AND/OR query logic
- Better performance with field projection
- Enhanced security with query limits
- Automatic join resolution
- Relationship preloading
- Soft delete handling
- More flexible and powerful queries
