# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Running the Application
- `make serve` - Builds and runs the application using Docker Compose on port 8080
- Direct Go execution: `go run . -port 8080 -dbfile data/purchases.db`
- Environment variable: `YNAB_TOKEN` - YNAB Personal Access Token for API integration

### Building
- `make build` - Build the Go binary
- `docker-compose up --build` - Build and run with Docker

### Code Generation
- `go generate ./...` - Regenerates YNAB API client from OpenAPI spec
- Located in `internal/ynab/`: uses oapi-codegen to generate client from https://api.ynab.com/papi/open_api_spec.yaml

## Architecture Overview

This is a Go web application that scrapes Amazon purchase data and integrates with YNAB (You Need A Budget) for transaction matching and categorization.

### Core Components

**Main Application (`main.go`)**
- HTTP server with two main routes:
  - `/api/*` - REST API endpoints (handled by `internal/api`)
  - `/` - Web UI (handled by `internal/ui`)
- Command-line flags for port, database file, and YNAB configuration

**Data Layer (`internal/store/`)**
- SQLite database interaction with transactions support
- `Store` struct handles purchase data persistence
- Search functionality by price, amount, card, item, or date
- Bulk category updates for YNAB integration

**Models (`internal/models/`)**
- `Order` - Amazon purchase with items, price, and charge details
- `Charge` - Payment card transaction with amount and date
- `UnapprovedTransaction`, `Category`, `Budget` - YNAB-specific types
- Date parsing and comparison utilities

**YNAB Integration (`internal/ynab/`)**
- Generated API client using oapi-codegen
- Handles authentication via Bearer tokens
- Functions for retrieving unapproved transactions, categories, budgets
- Transaction approval and categorization

**Web UI (`internal/ui/`)**
- Embedded static assets and HTML templates
- Template-driven interface for purchase data display
- Integration with YNAB for transaction matching

### Key Patterns

- Uses embedded file systems (`embed.FS`) for static assets and templates
- Repository pattern with interfaces for data access and YNAB operations
- Transaction-based database operations with rollback support
- Generated code for external API integration
- Docker multi-stage builds for minimal production images

### Database Schema

The application uses SQLite with these main tables:
- `purchases` - Order details with card and amount information
- `items` - Individual purchase items
- `purchase_items` - Junction table linking purchases to items
- `purchase_category` - YNAB category assignments for purchases

### Frontend

- Vanilla JavaScript with no build step (as per project goals)
- Uses Bulma CSS framework from CDN
- use HTMX to minimize client-side JS
- bookmarklet functionality in `internal/ui/static/export.js` for adding automation to amazon.com