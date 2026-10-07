.DEFAULT_GOAL := help
MODULES := shared go-pennywise-api go-gmail cipher workflows
.PHONY: help setup dev stop check check-go check-web smoke status logs
help:
	@echo "setup | dev | stop | check | smoke | status | logs (see docs/development.md)"
setup:
	@command -v go >/dev/null && command -v node >/dev/null && command -v docker >/dev/null
	npm --prefix react-frontend ci
	cd react-frontend && npx playwright install chromium
	@for module in $(MODULES); do (cd backend/$$module && go mod download) || exit $$?; done
dev:
	./scripts/dev.sh up
stop:
	./scripts/dev.sh down
status:
	./scripts/dev.sh ps
logs:
	./scripts/dev.sh logs --tail=100
check: check-go check-web
check-go:
	@for module in $(MODULES); do echo "Checking $$module"; (cd backend/$$module && go vet ./... && go test -timeout 120s ./...) || exit $$?; done
check-web:
	npm --prefix react-frontend run lint
	npm --prefix react-frontend run build
smoke:
	./scripts/smoke.sh
