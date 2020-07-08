VERSION = $(shell git describe --tags --dirty --always)
GIT_COMMIT = $(shell git rev-parse HEAD)
REPO = github.com/leg100/stok
LOGLEVEL ?= info
DOCKER_IMAGE = leg100/stok-operator:$(VERSION)
OPERATOR_NAMESPACE ?= default
OPERATOR_RELEASE ?= stok-operator
WORKSPACE_NAMESPACE ?= default
ALL_CRD = ./deploy/crds/stok.goalspike.com_all_crd.yaml
GCP_SVC_ACC ?= terraform@automatize-admin.iam.gserviceaccount.com
KIND_CONTEXT ?= kind-kind
GKE_CONTEXT ?= gke-stok
CLI_BIN ?= build/_output/bin/stok
LD_FLAGS = " \
	-X '$(REPO)/version.Version=$(VERSION)' \
	-X '$(REPO)/version.Commit=$(GIT_COMMIT)' \
	" \

.PHONY: local kind-deploy kind-context deploy-crds undeploy \
	create-namespace create-secret \
	e2e e2e-run \
	gcp-deploy \
	clean delete-command-resources delete-crds \
	unit \
	cli-unit cli-build cli-test \
	operator-build operator-image operator-load-image operator-unit \
	generate-all generate generate-deepcopy generate-crds generate-clientset

local:
	operator-sdk run --local \
		--operator-flags "--zap-level $(LOGLEVEL)" \
		--watch-namespace=default \
		--verbose 2>&1 \
		| jq -R -r '. as $$line | try fromjson catch $$line'

kind-context:
	kubectl config use-context $(KIND_CONTEXT)

gke-context:
	kubectl config use-context $(GKE_CONTEXT)

deploy-operator: cli-build
	$(CLI_BIN) generate operator | kubectl apply -f -
	kubectl rollout status deployment/stok-operator

undeploy-operator: cli-build
	$(CLI_BIN) generate operator | kubectl delete -f - --wait --ignore-not-found=true

deploy-crds: cli-build
	$(CLI_BIN) generate crds --local | kubectl create -f -

delete-crds: cli-build
	$(CLI_BIN) generate crds --local | kubectl delete -f -

create-namespace:
	kubectl get ns $(WORKSPACE_NAMESPACE) > /dev/null 2>&1 || kubectl create ns $(NAMESPACE)

create-secret:
	kubectl --namespace $(WORKSPACE_NAMESPACE) get secret stok || \
		kubectl --namespace $(WORKSPACE_NAMESPACE) create secret generic stok \
			--from-file=google-credentials.json=$(GOOGLE_APPLICATION_CREDENTIALS)

e2e: cli-build operator-image operator-load-image kind-context deploy-crds \
	deploy-operator create-namespace create-secret e2e-run e2e-clean

e2e-clean: delete-custom-resources undeploy-operator delete-crds

e2e-run:
	operator-sdk test local --operator-namespace $(OPERATOR_NAMESPACE) --verbose \
		--no-setup ./test/e2e

# delete all stok custom resources
delete-custom-resources:
	kubectl delete -n $(WORKSPACE_NAMESPACE) --all $$(kubectl api-resources \
		--api-group=stok.goalspike.com -o name \
		| tr '\n' ',' | sed 's/.$$//') || true

# delete all stok custom resources except workspace
delete-command-resources:
	kubectl delete -n $(WORKSPACE_NAMESPACE) --all $$(kubectl api-resources \
		--api-group=stok.goalspike.com -o name | grep -v workspaces \
		| tr '\n' ',' | sed 's/.$$//') || true

unit: operator-unit cli-unit

build: cli-build operator-build

cli-unit:
	go test -v ./cmd

cli-build:
	go build -o $(CLI_BIN) -ldflags $(LD_FLAGS) github.com/leg100/stok

install: cli-install

cli-install:
	go install -ldflags $(LD_FLAGS) github.com/leg100/stok

operator-build:
	go build -o stok-operator -ldflags $(LD_FLAGS) github.com/leg100/stok/cmd/manager

operator-image: operator-build
	docker build -f build/Dockerfile -t $(DOCKER_IMAGE) .

operator-push: operator-image
	docker push $(DOCKER_IMAGE) | tee push.out
	grep -o 'sha256:[a-f0-9]*' push.out > stok-operator.digest

operator-load-image:
	kind load docker-image $(DOCKER_IMAGE)

operator-unit:
	go test -v ./pkg/...

generate-all: generate generate-crds generate-deepcopy generate-clientset

generate:
	go generate ./...

generate-deepcopy:
	operator-sdk generate k8s

generate-crds:
	operator-sdk generate crds
	# add app=stok label to each crd
	@for f in ./deploy/crds/*_crd.yaml; do \
		kubectl label --overwrite -f $$f --local=true -oyaml app=stok > crd_with_label.yaml; \
		mv crd_with_label.yaml $$f; \
	done
	# combine crd yamls into one
	sed -se '$$s/$$/\n---/' ./deploy/crds/*_crd.yaml | head -n-1 > $(ALL_CRD)

generate-clientset:
	mkdir -p hack
	sed -e 's,^,// ,; s,  *$$,,' LICENSE > hack/boilerplate.go.txt

	rm -rf pkg/client/clientset
	go run k8s.io/code-generator/cmd/client-gen \
		--clientset-name clientset \
		--input-base github.com/leg100/stok/pkg/apis \
		--input stok/v1alpha1 \
		-h hack/boilerplate.go.txt \
		-p github.com/leg100/stok/pkg/client/

	mkdir -p pkg/client
	mv github.com/leg100/stok/pkg/client/clientset pkg/client/
	rm -rf github.com
