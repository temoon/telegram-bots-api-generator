# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a code generator that parses the official Telegram Bots API documentation and generates Go client libraries. It consists of two main parts:

1. **Generator** (root directory): Scrapes Telegram's API documentation and generates Go code
2. **Generated API Library** (`api/` directory): The generated Go client library for Telegram Bots API

## Architecture

### Generator Flow

The generator follows this pipeline:

1. **Fetch** (`parser.go:fetch`): Downloads HTML from https://core.telegram.org/bots/api
2. **Parse** (`parser.go:parse`): Extracts API methods and types from HTML structure
   - Methods: lowercase h4 headings → generates request structs
   - Types: uppercase h4 headings → generates type structs
   - Handles polymorphic types via "or" syntax (e.g., "Integer or String" → ChatId)
3. **Generate** (`generate.go`):
   - `generateTypes()`: Creates `api/types.go` with all Telegram types
   - `generateRequests()`: Creates individual files in `api/requests/` for each API method
     - `generateRequestFile()`: Generates `{method}.go` with implementation
     - `generateRequestTestFile()`: Generates `{method}_test.go` with unit tests
     - Creates `helpers_test.go` with shared test utilities

### Key Data Structures

- **Types**: Telegram object definitions (Message, User, Chat, etc.)
- **Methods**: API endpoints (sendMessage, getUpdates, etc.)
- **Fields**: Parameters/properties with type information and optional/required status

### Template System

Templates in `templates/` directory:
- `types_header.tmpl`: Package declaration for types.go
- `types.tmpl`: Struct definition template for each type
- `request.tmpl`: Complete request file template with Call/GetValues/GetFiles methods
- `request_test.tmpl`: Test file template for testing GetValues() and GetFiles() methods
- `helpers_test.tmpl`: Test helpers template (generates ptr[T] function for pointer creation)

### Type Mapping

The generator handles several special type conversions:

- `InputFile or String` → `InputFile` (file upload fields)
- `Integer or String` → `ChatId` (chat identifier that can be numeric ID or username)
- `Array of X` → `[]X` (Go slices)
- `X or Y or Z` → `interface{}` with runtime type switching in GetValues()

### Generated Code Structure

Each request in `api/requests/` implements three methods:

1. `Call(ctx, bot)`: Executes the request and returns response
2. `GetValues()`: Converts struct fields to string map for form encoding
3. `GetFiles()`: Extracts file attachments from InputFile fields (handles nested structures)

### Test Generation

The generator automatically creates comprehensive unit tests for all API methods:

#### Test Files Generated

- **`api/requests/helpers_test.go`**: Common test utilities
  - `ptr[T](v T) *T`: Generic helper function for creating pointers to values (used for optional fields in tests)

- **`api/requests/{method_name}_test.go`**: Test file for each API method with two test functions:
  - `Test{MethodName}_GetValues`: Validates field serialization to `map[string]string`
  - `Test{MethodName}_GetFiles`: Validates file extraction logic

#### GetValues Test Coverage

Tests verify correct conversion of struct fields to API parameters:

- **Required fields test case**: Creates request with only required fields, verifies all are present in output
- **Optional fields test case**: Creates request with both required and optional fields, verifies correct serialization
- **Type-specific serialization checks**:
  - Strings: passed through as-is
  - Numbers (`int64`, `float64`): converted to string representation
  - Booleans: converted to `"1"` (true) or `"0"` (false)
  - `ChatId`: serialized to numeric ID or username string
  - `InputFile`: serialized to file_id, URL, or `attach://` reference
  - Complex types (objects/arrays): JSON-marshaled to string

#### GetFiles Test Coverage

Tests verify file extraction behavior:

- **For requests with InputFile fields**:
  - Test with actual file reader: verifies files are extracted correctly
  - Test with file_id: verifies no files returned (file_id uses form encoding, not multipart)

- **For requests without InputFile fields**:
  - Verifies `GetFiles()` returns nil or empty map

#### Test Generation Logic

The generator intelligently creates tests based on field types:

- Skips test cases for interface{} fields (unpredictable test data)
- Skips test cases for complex objects without special handling (not testable with simple values)
- Creates meaningful test data for each field type:
  - Strings: `"test_{field_name}"`
  - Integers: `123` (required), `456` (optional)
  - Floats: `123.45` (required), `456.78` (optional)
  - Booleans: `true`
  - ChatId: numeric ID `123456` (required), `789` (optional)
  - InputFile: file_id `"file_id_123"` or reader with fake data

## Commands

### Generate API Code

```bash
go generate
```

This runs the generator (via `//go:generate` directive in generate.go:3-5) and formats the output.

### Update to Latest Telegram API

```bash
go run .
```

Re-fetches the Telegram API documentation and regenerates all files in `api/`.

### Working with the Generated Library

The `api/` directory is a separate Go module that can be used independently:

```bash
cd api
go mod tidy
```

### Run Tests

The generator creates comprehensive unit tests for all API methods:

```bash
# Run all tests in the generated API
cd api
go test ./...

# Run tests for requests package specifically
go test ./requests

# Run with verbose output
go test -v ./requests

# Run with coverage
go test -cover ./requests
```

## Important Notes

- The generator completely **removes and recreates** `api/requests/` on each run (including all `*.go` and `*_test.go` files)
- `api/types.go` is overwritten entirely each time
- The generator uses `go.mod` replace directive to reference the local `api/` module
- Field ordering in generated structs: required fields first, then optional (sorted alphabetically within each group)
- The `api/bot.go` file is **not generated** - it's hand-written infrastructure code for making HTTP requests
- Templates use Go's text/template with custom helper functions from `helpers.go`
- All tests in `api/requests/` are automatically generated - manual test files should be placed elsewhere

## File Upload Handling

InputFile fields require special handling:

- Fields marked with link to "#sending-files" in Telegram docs become `InputFile` type
- Generator recursively scans for InputFile fields in nested types and arrays
- `GetFiles()` method extracts all file readers using type switching for polymorphic fields
- Supports files in: direct fields, array elements, union type variants, and nested objects
