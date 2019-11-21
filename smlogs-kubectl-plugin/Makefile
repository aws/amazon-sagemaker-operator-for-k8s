build: fmt vet
	go build -o bin/kubectl-smlogs cmd/kubectl-smlogs.go

build-release:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -o bin/kubectl-smlogs.darwin.amd64 cmd/kubectl-smlogs.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/kubectl-smlogs.linux.amd64 cmd/kubectl-smlogs.go

test: fmt vet 
	go test -v ./pkg/... -coverprofile cover.out

run: fmt vet
	go run ./cmd/kubectl-smlogs.go

fmt:
	go fmt ./...

vet:
	go vet ./...
