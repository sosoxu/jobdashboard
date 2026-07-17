.PHONY: all bff frontend clean run-dev

# Build frontend then bundle it into the BFF static dir, then build the BFF.
all: frontend bff

frontend:
	cd frontend && npm install --no-audit --no-fund
	cd frontend && npm run build
	rm -rf bff/web/dist
	mkdir -p bff/web
	cp -r frontend/dist bff/web/dist

bff:
	cd bff && go mod tidy
	cd bff && go build -o bin/bff ./cmd/server

run-dev:
	@echo "In one terminal: cd bff && go run ./cmd/server"
	@echo "In another:     cd frontend && npm run dev"

clean:
	rm -rf bff/bin bff/data bff/web frontend/dist frontend/node_modules
