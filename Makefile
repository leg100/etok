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
.PHONY: deploy-operator
deploy-operator: image push
	$(BUILD_BIN) generate operator --local --image $(IMG) | $(KUBECTL) apply -f -
	$(KUBECTL) rollout status --timeout=10s deployment/etok-operator

.PHONY: delete-operator
delete-operator: build
	$(BUILD_BIN) generate operator --local | $(KUBECTL) delete -f - --wait --ignore-not-found=true

.PHONY: deploy-crds
deploy-crds: build manifests
	$(BUILD_BIN) generate crds --local | $(KUBECTL) create -f -

.PHONY: delete-crds
delete-crds: build
	$(BUILD_BIN) generate crds --local | $(KUBECTL) delete -f - --ignore-not-found

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
e2e: image push deploy-crds deploy-operator create-namespace create-secret e2e-run e2e-clean

.PHONY: e2e-clean
e2e-clean: delete-workspaces delete-operator delete-crds delete-secret

.PHONY: e2e-run
e2e-run:
	go test -v ./test/e2e -context $(KUBECTX)

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

# delete all etok workspaces
.PHONY: delete-workspaces
delete-workspaces: build
	# Using etok bin rather than kubectl because etok bin will wait for workspaces' dependents
	# to be deleted first before deleting the workspace itself.
	$(BUILD_BIN) workspace list | awk '{ print $$NF }' | xargs -IWS $(BUILD_BIN) workspace delete WS

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
	@{ \
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=etok-operator webhook paths="./..." output:crd:artifacts:config=config/crd/bases;\
	for f in ./config/crd/bases/*.yaml; do \
		$(KUBECTL) label --overwrite -f $$f --local=true -oyaml app=etok > crd_with_label.yaml;\
 		mv crd_with_label.yaml $$f;\
 	done;\
 	sed -se '$$s/$$/\n---/' ./config/crd/bases/*.yaml | head -n-1 > $(ALL_CRD);\
	}

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
