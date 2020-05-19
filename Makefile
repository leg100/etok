LOGLEVEL ?= info
NAMESPACE ?= operator-test
IMAGE_TAG ?= latest
RELEASE ?= stok
ALL_CRD = ./deploy/crds/stok.goalspike.com_all_crd.yaml
GCP_SVC_ACC ?= terraform@automatize-admin.iam.gserviceaccount.com
KIND_CONTEXT ?= kind-kind
GKE_CONTEXT ?= gke-stok

.PHONY: local kind-deploy kind-context deploy-crds undeploy \
	create-namespace create-secret \
	e2e e2e-run \
	gcp-deploy \
	delete-command-resources \
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
	kubectl config use-context kind-kind

kind-deploy:
	helm upgrade -i $(RELEASE) ./charts/stok/ \
		--kube-context kind-kind \
		--wait --timeout 1m \
		--set image.tag=$(IMAGE_TAG) --namespace $(NAMESPACE)

gcp-deploy:
	helm upgrade -i $(RELEASE) ./charts/stok/ \
		--kube-context gke-stok \
		--wait --timeout 1m \
		--set image.digest=$$(cat stok-operator.digest) \
		--set image.pullPolicy=Always \
		--set workloadIdentity=true \
		--set gcpServiceAccount=$(GCP_SVC_ACC) \
		--set cache.storageClass=local-path \
		--namespace default

deploy-crds:
	kubectl --namespace $(NAMESPACE) apply -f $(ALL_CRD)

delete-crds:
	kubectl delete crds --all

undeploy:
	helm delete $(RELEASE) --namespace $(NAMESPACE) || true

create-namespace:
	kubectl get ns $(NAMESPACE) || kubectl create ns $(NAMESPACE)

create-secret:
	kubectl --namespace $(NAMESPACE) create secret generic stok \
		--from-file=google-credentials.json=$(KEY)

e2e: cli-build operator-image operator-load-image kind-context \
	create-namespace delete-command-resources undeploy delete-crds kind-deploy e2e-run undeploy

e2e-run:
	operator-sdk test local --operator-namespace $(NAMESPACE) --verbose \
		--no-setup ./test/e2e

# delete all stok custom resources except workspace
delete-command-resources:
	kubectl delete -n $(NAMESPACE) --all $$(kubectl api-resources \
		| awk '$$2 == "stok.goalspike.com" && $$1 != "workspaces" { print $$1 }' \
		| tr '\n' ',' | sed 's/.$$//') || true

unit: operator-unit cli-unit

cli-unit:
	go test -v ./cmd

cli-build:
	go build -o build/_output/bin/stok github.com/leg100/stok

operator-build:
	go build -o stok-operator github.com/leg100/stok/cmd/manager

operator-image: operator-build
	docker build -f build/Dockerfile -t leg100/stok-operator:latest .

operator-push: operator-image
	docker push leg100/stok-operator:latest | tee push.out
	grep -o 'sha256:[a-f0-9]*' push.out > stok-operator.digest

operator-load-image:
	kind load docker-image leg100/stok-operator:latest

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
