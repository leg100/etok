VERSION = $(shell git describe --tags --dirty --always)
GIT_COMMIT = $(shell git rev-parse HEAD)
REPO = github.com/leg100/etok
RANDOM_SUFFIX := $(shell cat /dev/urandom | tr -dc 'a-z0-9' | head -c5)
BUILD_BIN ?= ./etok
KUBECTL = kubectl --context=$(KUBECTX)
KUBE_VERSION = v0.18.2
LD_FLAGS = " \
	-X '$(REPO)/pkg/version.Version=$(VERSION)' \
	-X '$(REPO)/pkg/version.Commit=$(GIT_COMMIT)' \
	" \

ENV ?= kind
IMG ?= etok
TAG ?= $(VERSION)-$(RANDOM_SUFFIX)
KUBECTX=""

# Override vars if ENV=gke
ifeq ($(ENV),gke)
IMG = $(GKE_IMAGE)
DEPLOY_FLAGS = --sa-annotations iam.gke.io/gcp-service-account=$(BACKUP_SERVICE_ACCOUNT)
KUBECTX = $(GKE_KUBE_CONTEXT)
endif

# Override vars if ENV=kind
ifeq ($(ENV),kind)
KUBECTX=kind-kind
endif

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Even though operator runs outside the cluster, it still creates pods. So an image still needs to
# be built and pushed/loaded first.
.PHONY: local
local: image push
	ETOK_IMAGE=$(IMG):$(TAG) $(BUILD_BIN) operator --context $(KUBECTX)

# Same as above - image still needs to be built and pushed/loaded
.PHONY: deploy
deploy: image push deploy-operator-secret
	$(BUILD_BIN) install --context $(KUBECTX) --local --image $(IMG):$(TAG) $(DEPLOY_FLAGS) \
		--backup-provider=gcs --gcs-bucket=$(BACKUP_BUCKET)

# Tail operator logs
.PHONY: logs
logs:
	$(KUBECTL) --namespace=etok logs -f deploy/etok

# Deploy only CRDs
.PHONY: crds
crds: build
	$(BUILD_BIN) install --context $(KUBECTX) --local --crds-only

.PHONY: undeploy
undeploy: build delete-operator-secret
	$(BUILD_BIN) install --local --dry-run | $(KUBECTL) delete -f - --wait --ignore-not-found=true


# Deploy a secret containing GCP svc acc key, on kind, for the operator to use
.PHONY: deploy-operator-secret
deploy-operator-secret: delete-operator-secret create-operator-namespace
ifeq ($(ENV),kind)
	$(KUBECTL) --namespace=etok create secret generic etok --from-file=GOOGLE_CREDENTIALS=$(GOOGLE_APPLICATION_CREDENTIALS)
endif

.PHONY: delete-operator-secret
delete-operator-secret:
ifeq ($(ENV),kind)
	$(KUBECTL) --namespace=etok delete secret etok --ignore-not-found
endif

# Create operator namespace, ignore already exists errors
.PHONY: create-operator-namespace
create-operator-namespace:
ifeq ($(ENV),kind)
	$(KUBECTL) create namespace etok 2>/dev/null || true
endif

.PHONY: e2e
e2e: image push deploy
	go test -v ./test/e2e -failfast -context $(KUBECTX)

.PHONY: unit
unit:
	go test ./ ./cmd/... ./pkg/...

.PHONY: build
build:
	CGO_ENABLED=0 go build -o $(BUILD_BIN) -ldflags $(LD_FLAGS) github.com/leg100/etok

.PHONY: install
install:
	go install -ldflags $(LD_FLAGS) github.com/leg100/etok

.PHONY: install-latest-release
install-latest-release:
	curl -s https://api.github.com/repos/leg100/etok/releases/latest \
		| jq -r '.assets[] | select(.name | test(".*_linux_amd64$$")) | .browser_download_url' \
		| xargs -I{} curl -Lo /tmp/etok {}
	chmod +x /tmp/etok
	mv /tmp/etok ~/go/bin/

.PHONY: image
image: build
	docker build -f build/Dockerfile -t $(IMG):$(TAG) .

.PHONY: push
push:
ifeq ($(ENV),kind)
	kind load docker-image $(IMG):$(TAG)
else
	docker push $(IMG):$(TAG)
endif

# Generate manifests e.g. CRD, RBAC etc.
# add app=etok label to each crd
# combine crd yamls into one
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=etok webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# Generate code (deep-copy funcs)
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# find or download controller-gen
# download controller-gen if necessary
.PHONY: local
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

.PHONY: generate-clientset
generate-clientset: client-gen
	@{ \
	set -e ;\
	rm -rf pkg/k8s/etokclient ;\
	$(CLIENT_GEN) \
		--clientset-name etokclient \
		--input-base github.com/leg100/etok/api \
		--input etok.dev/v1alpha1 \
		-h hack/boilerplate.go.txt \
		-p github.com/leg100/etok/pkg/k8s ;\
	mv github.com/leg100/etok/pkg/k8s/etokclient pkg/k8s/ ;\
	rm -rf github.com ;\
	}

# find or download client-gen
# download client-gen if necessary
.PHONY: client-gen
client-gen:
ifeq (, $(shell which client-gen))
	@{ \
	set -e ;\
	CLIENT_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CLIENT_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get k8s.io/code-generator/cmd/client-gen@$(KUBE_VERSION) ;\
	rm -rf $$CLIENT_GEN_TMP_DIR ;\
	}
CLIENT_GEN=$(GOBIN)/client-gen
else
CLIENT_GEN=$(shell which client-gen)
endif

.PHONY: kustomize
kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif
