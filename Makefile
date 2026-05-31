.PHONY: test docs

test:
	go test -timeout=5m $(shell go list ./...)

# Regenerate API docs: Swagger 2.0 from code annotations, then OpenAPI v3.
# The pinned swag version is run module-isolated (the @version form) so its
# CLI-only dependencies don't have to be tracked in this module's go.sum.
SWAG_VERSION := v1.16.6
docs:
	go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION) init --parseDependency --parseInternal
	go run ./cmd/swagger2openapi
