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

### Key Data Structures

- **Types**: Telegram object definitions (Message, User, Chat, etc.)
- **Methods**: API endpoints (sendMessage, getUpdates, etc.)
- **Fields**: Parameters/properties with type information and optional/required status

### Template System

Templates in `templates/` directory:
- `types_header.tmpl`: Package declaration for types.go
- `types.tmpl`: Struct definition template for each type
- `request.tmpl`: Complete request file template with Call/GetValues/GetFiles methods

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

## Important Notes

- The generator completely **removes and recreates** `api/requests/` on each run
- `api/types.go` is overwritten entirely each time
- The generator uses `go.mod` replace directive to reference the local `api/` module
- Field ordering in generated structs: required fields first, then optional (sorted alphabetically within each group)
- The `api/bot.go` file is **not generated** - it's hand-written infrastructure code for making HTTP requests
- Templates use Go's text/template with custom helper functions from `helpers.go`

## File Upload Handling

InputFile fields require special handling:

- Fields marked with link to "#sending-files" in Telegram docs become `InputFile` type
- Generator recursively scans for InputFile fields in nested types and arrays
- `GetFiles()` method extracts all file readers using type switching for polymorphic fields
- Supports files in: direct fields, array elements, union type variants, and nested objects
