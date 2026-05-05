# Changelog

All notable changes to sql-generator will be documented in this file.

## [1.0.0] - 2026-05-05

Initial public release.

### Features

#### Filtering
- 14 filter operators: `IS_EQUAL`, `IS_NOT_EQUAL`, `IS_LESS_THAN`, `IS_MORE_THAN`, `IS_LESS_THAN_OR_EQUAL`, `IS_MORE_THAN_OR_EQUAL`, `IS_CONTAIN`, `IS_BEGIN_WITH`, `IS_END_WITH`, `IS_BETWEEN`, `IS_IN`, `IS_NOT_IN`, `IS_NULL`, `IS_NOT_NULL`
- Simple flat filters (all ANDed together)
- Complex `FilterGroup` with unlimited AND/OR nesting

#### Search & Sort
- Global search across `Searchable` fields (LIKE or ILIKE)
- `CaseSensitiveSearch` toggle for case-insensitive search
- Multi-field sorting with `ASCENDING` / `DESCENDING` per field
- `DefaultFieldForSort` fallback when no sort is specified

#### Data Retrieval
- Offset-based pagination with configurable `Page` and `PageSize` (defaults: page 1, size 20)
- Field projection — SELECT specific columns via `Fields []string`
- Automatic JOIN resolution based on `JoinMeta` field configuration
- Eager loading via `Preloads []string` (GORM `Preload`)
- Soft delete support — `IncludeDeleted` flag calls GORM `Unscoped()`
- `Distinct` flag for deduplication

#### Security
- Field whitelisting via `ModelMeta.Fields` — unknown fields are rejected
- Per-field operator whitelisting via `FieldMeta.Operators`
- `MaxFiltersPerQuery` and `MaxSortsPerQuery` limits with recursive filter counting
- All values passed as GORM parameterized arguments (no string concatenation)

#### HTTP Binding Layer
- `binding.ParseBody(r io.Reader)` — framework-agnostic JSON body parser (net/http, Gin)
- `binding.ParseBytes(b []byte)` — byte-slice variant (Fiber, Echo)
- All model structs have `snake_case` JSON tags for direct body binding

#### Examples
- 10 usage examples in `example/example.go`
- Gin, Fiber, and net/http handler examples in `example/http_example.go`
