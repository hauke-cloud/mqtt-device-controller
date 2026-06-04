LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.0
ENVTEST ?= go run sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
ENVTEST_K8S_VERSION ?= 1.31.0

.PHONY: all build test lint generate manifests setup-envtest docker-build clean fmt vet

all: generate manifests build

build: fmt vet
	CGO_ENABLED=0 go build -o $(LOCALBIN)/controller ./cmd/controller/

test: setup-envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

lint: fmt vet

fmt:
	gofmt -s -l .

vet:
	go vet ./...

generate:
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

manifests:
	$(CONTROLLER_GEN) crd rbac:roleName=mqtt-device-controller-role \
		paths="./api/..." \
		output:crd:artifacts:config=config/crd/bases

setup-envtest:
	@mkdir -p $(LOCALBIN)
	$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN)

docker-build:
	docker build -t mqtt-device-controller:latest .

clean:
	rm -rf $(LOCALBIN) coverage.out
