GOPATH=$(shell pwd)/vendor:$(shell pwd)

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=goburstpool

start:
	make build
	./$(BINARY_NAME)
api:
	protoc --go_out=plugins=grpc:src/ api/api.proto
build: deps
	@GOPATH=$(GOPATH) $(GOBUILD) -o $(BINARY_NAME)
deps:
	@GOPATH=$(GOPATH) $(GOGET) github.com/gorilla/websocket
	@GOPATH=$(GOPATH) $(GOGET) gopkg.in/yaml.v2
	@GOPATH=$(GOPATH) $(GOGET) github.com/jinzhu/gorm/dialects/mysql
	@GOPATH=$(GOPATH) $(GOGET) github.com/stretchr/testify
	@GOPATH=$(GOPATH) $(GOGET) github.com/spebern/globa
	@GOPATH=$(GOPATH) $(GOGET) github.com/throttled/throttled
	@GOPATH=$(GOPATH) $(GOGET) github.com/throttled/throttled/store/memstore
	@GOPATH=$(GOPATH) $(GOGET) go.uber.org/zap
	@GOPATH=$(GOPATH) $(GOGET) github.com/satori/go.uuid
	@GOPATH=$(GOPATH) $(GOGET) github.com/jmoiron/sqlx
	@GOPATH=$(GOPATH) $(GOGET) github.com/klauspost/cpuid
	@GOPATH=$(GOPATH) $(GOGET) google.golang.org/grpc
	@GOPATH=$(GOPATH) $(GOGET) golang.org/x/net/context
mocks:
	@GOPATH=$(GOPATH) mockery -name=Wallet -dir=./src/wallet/
test:
	go test ./... -cover
cover:
	go test ./... -coverprofile=cover.out
	go tool cover -html=cover.out
