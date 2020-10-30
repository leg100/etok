VERSION = $(shell git describe --tags --dirty --always)
GIT_COMMIT = $(shell git rev-parse HEAD)
REPO = github.com/leg100/stok
RANDOM_SUFFIX := $(shell cat /dev/urandom | tr -dc 'a-z0-9' | head -c5)
WORKSPACE_NAMESPACE ?= default
ALL_CRD = ./config/crd/bases/stok.goalspike.com_all.yaml
BUILD_BIN ?= ./stok
KUBECTL = kubectl --context=$(KUBECTX)
KUBE_VERSION=v0.18.2
LD_FLAGS = " \
	-X '$(REPO)/version.Version=$(VERSION)' \
	-X '$(REPO)/version.Commit=$(GIT_COMMIT)' \
	" \

ifeq ($(ENV),gke)
KUBECTX=gke_automatize-admin_europe-west2-a_stok
IMG=eu.gcr.io/automatize-admin/stok:$(VERSION)-$(RANDOM_SUFFIX)
else
KUBECTX=kind-kind
IMG=leg100/stok:$(VERSION)
endif

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: local kind-deploy kind-context deploy-crds delete \
	create-namespace create-secret \
	e2e e2e-run \
	clean delete-command-resources delete-crds \
	unit \
	install install-latest-release \
	build cli-test cli-install \
	operator-build image push \
	manifests \
	generate \
	controller-gen \
	fmt vet \
	kustomize

# Even though operator runs outside the cluster, it still creates pods. So an image still needs to
# be built and pushed/loaded first.
local: image push
	STOK_IMAGE=$(IMG) $(BUILD_BIN) operator --context $(KUBECTX)

# Same as above - image still needs to be built and pushed/loaded
deploy-operator: image push
	$(BUILD_BIN) generate operator --image $(IMG) | $(KUBECTL) apply -f -
	$(KUBECTL) rollout status --timeout=10s deployment/stok-operator

delete-operator: build
	$(BUILD_BIN) generate operator | $(KUBECTL) delete -f - --wait --ignore-not-found=true

deploy-crds: build manifests
	$(BUILD_BIN) generate crds --local | $(KUBECTL) create -f -

delete-crds: build
	$(BUILD_BIN) generate crds --local | $(KUBECTL) delete -f - --ignore-not-found

create-namespace:
	$(KUBECTL) get ns $(WORKSPACE_NAMESPACE) > /dev/null 2>&1 || $(KUBECTL) create ns $(NAMESPACE)

create-secret:
	$(KUBECTL) --namespace $(WORKSPACE_NAMESPACE) get secret stok || \
		$(KUBECTL) --namespace $(WORKSPACE_NAMESPACE) create secret generic stok \
			--from-file=google-credentials.json=$(GOOGLE_APPLICATION_CREDENTIALS)

delete-secret:
	$(KUBECTL) --namespace $(WORKSPACE_NAMESPACE) delete secret stok --ignore-not-found=true

e2e: image push deploy-crds deploy-operator create-namespace create-secret e2e-run e2e-clean

e2e-clean: delete-custom-resources delete-operator delete-crds delete-secret

e2e-run:
	go test -v ./test/e2e -context $(KUBECTX)

# delete all stok custom resources
delete-custom-resources:
	$(KUBECTL) delete -n $(WORKSPACE_NAMESPACE) --all $$($(KUBECTL) api-resources \
		--api-group=stok.goalspike.com -o name \
		| tr '\n' ',' | sed 's/.$$//') || true

# delete all stok custom resources except workspace
delete-run-resources:
	$(KUBECTL) delete -n $(WORKSPACE_NAMESPACE) --all runs.stok.goalspike.com

unit:
	go test ./ ./cmd/... ./controllers/... ./pkg/... ./util/...

build:
	CGO_ENABLED=0 go build -o $(BUILD_BIN) -ldflags $(LD_FLAGS) github.com/leg100/stok

install:
	go install -ldflags $(LD_FLAGS) github.com/leg100/stok

install-latest-release:
	curl -s https://api.github.com/repos/leg100/stok/releases/latest \
		| jq -r '.assets[] | select(.name | test(".*_linux_amd64$$")) | .browser_download_url' \
		| xargs -I{} curl -Lo /tmp/stok {}
	chmod +x /tmp/stok
	mv /tmp/stok ~/go/bin/

image: build
	docker build -f build/Dockerfile -t $(IMG) .

push:
ifeq ($(ENV),gke)
	docker push $(IMG)
else
	kind load docker-image $(IMG)
endif

# Generate manifests e.g. CRD, RBAC etc.
# add app=stok label to each crd
# combine crd yamls into one
manifests: controller-gen
	@{ \
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases;\
	for f in ./config/crd/bases/*.yaml; do \
		$(KUBECTL) label --overwrite -f $$f --local=true -oyaml app=stok > crd_with_label.yaml;\
 		mv crd_with_label.yaml $$f;\
 	done;\
 	sed -se '$$s/$$/\n---/' ./config/crd/bases/*.yaml | head -n-1 > $(ALL_CRD);\
	}

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code (deep-copy funcs)
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

generate-clientset: client-gen
	@{ \
	set -e ;\
	rm -rf pkg/k8s/stokclient ;\
	$(CLIENT_GEN) \
		--clientset-name stokclient \
		--input-base github.com/leg100/stok/api \
		--input stok.goalspike.com/v1alpha1 \
		-h hack/boilerplate.go.txt \
		-p github.com/leg100/stok/pkg/k8s ;\
	mv github.com/leg100/stok/pkg/k8s/stokclient pkg/k8s/ ;\
	rm -rf github.com ;\
	}

# find or download client-gen
# download client-gen if necessary
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
