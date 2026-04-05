API_NAME = gens6

GOLANGCILINT=golangci-lint

PROTOC=protoc
PROTOC_FLAGS=\
	-I api/proto \
	--go_out=. --go_opt=module=mko.re/$(API_NAME) --go_opt=default_api_level=API_OPAQUE

API_GO_DIR = api/$(API_NAME)pb
API_GO_FILES = $(API_GO_DIR)/$(API_NAME).pb.go
API_FILES = $(API_GO_FILES)

all: $(API_FILES)
	go build ./cmd/gens6

.PHONY: install-deps
install-deps: install-go-deps

.PHONY: install-go-deps
install-go-deps:
	go install tool
ifneq ($(CI),1)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v2.6.0
endif

.PHONY: gen
gen: gen-proto

.PHONY: gen-proto
gen-proto: $(API_FILES) 

$(API_GO_DIR)/%.pb.go: api/proto/mko_re/$(API_NAME)/%.proto
	$(PROTOC) $(PROTOC_FLAGS) $<

.PHONY: lint
lint: lint-go

.PHONY: lint-go
lint-go:
	go vet ./...
ifneq ($(CI),1)
	$(GOLANGCILINT) run
endif
