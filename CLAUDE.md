# huginn — Claude Guidelines

## Build & Test

```bash
make install        # Install dev tools (templ)
make docker-up      # Start PostgreSQL
# Migrations run automatically when the server starts
hamr dev            # Run dev server (file watching, builds, live reload)
make build          # Build binary (generates templ first)
make test           # Run tests
make lint           # Run linters
make templint       # Lint .templ files for silent failures and a11y issues
make docker-down    # Stop services
```

## Project Conventions

See AGENTS.md for all project conventions, patterns, and guides.

## Framework Reference

See `docs/llms.txt` for a compact HAMR API reference.
See `docs/llms-full.txt` for complete package documentation.
