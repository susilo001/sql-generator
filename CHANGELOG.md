# Changelog

All notable changes to sql-generator will be documented in this file.

## [2.0.0] - 2026-07-09

Second major release. Focus: URL-query parsing, FilterGroup correctness,
and hardening.

### Breaking Changes
- Removed JSON body binding helpers from `binding/binding.go`
- Replaced body-based binding flow with URL query parsing via `binding.ParseRequest`, `binding.ParseRawURL`, and `binding.ParseValues`
- Removed stale `IncludeDeleted` / `Distinct` references from examples and docs

### Features
- Added direct URL query parsing for:
  - `search`
  - `page`
  - `pageSize`
  - `sort=field:asc|desc`
  - `filter=field:value:op`
  - `fields`
  - `preloads`
- Added case-insensitive operator aliases for URL parsing:
  - `equals`, `notequals`, `greaterthan`, `greaterthanorequal`
  - `lessthan`, `lessthanorequal`, `contains`
  - `startswith`, `endswith`, `isin`, `isnotin`
- Added opt-in hardening controls:
  - `Generator.MaxFilterGroupDepth`
  - `Generator.MaxFilterGroupsPerQuery`
  - `Generator.MaxPageSize`
  - `Generator.MaxRangeValues`
  - `ModelMeta.AllowedPreloads`

### Fixes
- Fixed nested `FilterGroup` SQL generation so nested `AND` / `OR` logic is preserved correctly
- Unknown fields and disallowed operators inside nested `FilterGroup`s now fail with errors instead of being silently skipped
- Upgraded `github.com/jackc/pgx/v5` to `v5.9.2` to remove reachable SQL injection advisory `GO-2026-5004`

### Security
- Added security probe tests proving hostile filter values, search terms, IN-list values, field names, sort fields, and full URL payloads remain parameterized or rejected
- Added benchmark coverage for scope generation, URL parsing, end-to-end dry-run SQL rendering, and deep nesting behavior
- Preserved zero-value backward compatibility for new generator limits

### Documentation
- Rewrote `example/example.go` to match current features and new URL-binding flow
- Added package and symbol godoc across `generator.go`, `binding/url.go`, and `model/*.go`
- Corrected changelog/docs to remove features no longer present

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
