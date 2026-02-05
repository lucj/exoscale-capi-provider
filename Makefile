IMG ?= exoscale/cluster-api-provider-exoscale:latest
CONTROLLER_GEN = go run sigs.k8s.io/controller-tools/cmd/controller-gen

.PHONY: all
all: test

# ---------- Testing ----------
.PHONY: test
test: manifests generate fmt vet
	go test ./...

# ---------- Build ----------
.PHONY: build
build: generate fmt vet
	go build -o bin/manager ./cmd/manager/

# ---------- Manifest generation (CRDs, RBAC, webhooks) ----------
.PHONY: manifests
manifests:
	$(CONTROLLER_GEN) \
		rbac:roleName=manager-role \
		crd \
		webhook \
		paths="./..." \
		output:crd:artifacts:config=config/crd/bases \
		output:webhook:artifacts:config=config/webhook

# ---------- Code generation (deepcopy) ----------
.PHONY: generate
generate:
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# ---------- Formatting / Linting ----------
.PHONY: fmt
fmt:
	gofmt -l -w .

.PHONY: vet
vet:
	go vet ./...

# ---------- Docker ----------
.PHONY: docker-build
docker-build:
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push:
	docker push ${IMG}

# ---------- Clean ----------
.PHONY: clean
clean:
	rm -rf bin/
