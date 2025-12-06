HTMX_VERSION := 1.9.10
BULMA_VERSION := 0.9.4

STATIC_DIR := internal/ui/static
HTMX_FILE := $(STATIC_DIR)/htmx.min.js
BULMA_FILE := $(STATIC_DIR)/bulma.min.css

serve:
	DOCKER_BUILDKIT=1 docker-compose up --build

build:
	@mkdir -p bin
	go build -o bin/app

ci: build

# Download frontend dependencies
.PHONY: download-deps
download-deps: $(HTMX_FILE) $(BULMA_FILE)

$(HTMX_FILE):
	@mkdir -p $(STATIC_DIR)
	@echo "Downloading htmx $(HTMX_VERSION)..."
	@curl -fsSL -o $@ https://unpkg.com/htmx.org@$(HTMX_VERSION)/dist/htmx.min.js

$(BULMA_FILE):
	@mkdir -p $(STATIC_DIR)
	@echo "Downloading Bulma CSS $(BULMA_VERSION)..."
	@curl -fsSL -o $@ https://cdn.jsdelivr.net/npm/bulma@$(BULMA_VERSION)/css/bulma.min.css
