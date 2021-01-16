VERSION = $(shell git describe --tags --dirty --always)
GIT_COMMIT = $(shell git rev-parse HEAD)
REPO = github.com/leg100/etok
RANDOM_SUFFIX := $(shell cat /dev/urandom | tr -dc 'a-z0-9' | head -c5)
WORKSPACE_NAMESPACE ?= default
ALL_CRD = ./config/crd/bases/etok.dev_all.yaml
BUILD_BIN ?= ./etok
KUBECTL = kubectl --context=$(KUBECTX)
KUBE_VERSION=v0.18.2
LD_FLAGS = " \
	-X '$(REPO)/pkg/version.Version=$(VERSION)' \
	-X '$(REPO)/pkg/version.Commit=$(GIT_COMMIT)' \
	" \

ifeq ($(ENV),gke)
KUBECTX=gke_automatize-admin_europe-west2-a_etok-1
IMG=eu.gcr.io/automatize-admin/etok:$(VERSION)-$(RANDOM_SUFFIX)
else
KUBECTX=kind-kind
IMG=leg100/etok:$(VERSION)
endif
BACKUP_SERVICE_ACCOUNT=backup@automatize-admin.iam.gserviceaccount.com
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
	ETOK_IMAGE=$(IMG) $(BUILD_BIN) operator --context $(KUBECTX)

# Same as above - image still needs to be built and pushed/loaded
.PHONY: deploy
deploy: image push
ifeq ($(ENV),gke)
	# For GKE, use workload identity
	$(BUILD_BIN) install --context $(KUBECTX) --local --image $(IMG) --sa-annotations iam.gke.io/gcp-service-account=$(BACKUP_SERVICE_ACCOUNT)
else
	# All other clusters, place key in a secret resource
	$(BUILD_BIN) install --context $(KUBECTX) --local --image $(IMG) --secret-file $(GOOGLE_APPLICATION_CREDENTIALS)
endif

# Tail operator logs
.PHONY: logs
logs:
	$(KUBECTL) --namespace=etok logs -f deploy/etok

# Deploy only CRDs
.PHONY: crds
crds: build
	$(BUILD_BIN) install --context $(KUBECTX) --local --crds-only

.PHONY: undeploy
undeploy: build
	$(BUILD_BIN) install --local --dry-run | $(KUBECTL) delete -f - --wait --ignore-not-found=true

.PHONY: create-namespace
create-namespace:
	$(KUBECTL) get ns $(WORKSPACE_NAMESPACE) > /dev/null 2>&1 || $(KUBECTL) create ns $(NAMESPACE)

.PHONY: create-secret
create-secret:
	$(KUBECTL) --namespace $(WORKSPACE_NAMESPACE) get secret etok || \
		$(KUBECTL) --namespace $(WORKSPACE_NAMESPACE) create secret generic etok \
			--from-file=GOOGLE_CREDENTIALS=$(GOOGLE_APPLICATION_CREDENTIALS)

.PHONY: delete-secret
delete-secret:
	$(KUBECTL) --namespace $(WORKSPACE_NAMESPACE) delete secret etok --ignore-not-found=true

.PHONY: e2e
e2e: image push deploy e2e-run e2e-clean

.PHONY: e2e-clean
e2e-clean: undeploy

.PHONY: e2e-run
e2e-run:
	go test -v ./test/e2e -failfast -context $(KUBECTX)

# delete all etok custom resources (via kubectl)
.PHONY: delete-custom-resources
delete-custom-resources:
	$(KUBECTL) delete -n $(WORKSPACE_NAMESPACE) --all --wait $$($(KUBECTL) api-resources \
		--api-group=etok.dev -o name \
		| tr '\n' ',' | sed 's/.$$//') || true

# delete all etok custom resources except workspace
.PHONY: delete-run-resources
delete-run-resources:
	$(KUBECTL) delete -n $(WORKSPACE_NAMESPACE) --all runs.etok.dev

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
	docker build -f build/Dockerfile -t $(IMG) .

.PHONY: push
push:
ifeq ($(ENV),gke)
	docker push $(IMG)
else
	kind load docker-image $(IMG)
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
