.PHONY: mocks protos

start: build
	./Nogrod

protos:
	mkdir -p pkg/nodecom
	mkdir -p pkg/api
	protoc --go_out=plugins=grpc:pkg/ protos/nodecom.proto
	protoc --go_out=plugins=grpc:pkg/ protos/api.proto
	mv pkg/protos/nodecom.pb.go pkg/nodecom/
	mv pkg/protos/api.pb.go pkg/api/
	rm -r pkg/protos

api:
	protoc --go_out=plugins=grpc:pkg/ api/api.proto

build: libs
	go build -o Nogrod

build-docker:
	go build -o Nogrod

libs:
	cd pkg/burstmath && $(MAKE)

mocks:
	mockery -name=WalletHandler -dir=./pkg/wallethandler/
	mkdir -p src/mocks
	mv mocks/WalletHandler.go src/mocks/
	rm -r mocks

test:
	go test ./... -cover

cover:
	go test ./... -coverprofile=cover.out
	go tool cover -html=cover.out
