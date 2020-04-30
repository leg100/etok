.PHONY: local
local:
	operator-sdk run --local --watch-namespace=default --verbose 2>&1 \
		| jq -R -r '. as $$line | try fromjson catch $$line'

.PHONY: clean
clean:
	@echo ....... Deleting Rules and Service Account .......
	- kubectl delete -f deploy/role.yaml -n operator-test
	- kubectl delete -f deploy/role_binding.yaml -n operator-test
	- kubectl delete -f deploy/service_account.yaml -n operator-test
	@echo ....... Deleting Operator .......
	- kubectl delete -f deploy/operator.yaml -n operator-test
	@echo ....... Deleting test CRs .......
	- kubectl delete commands.stok.goalspike.com example-command -n operator-test
	- kubcetl delete workspaces.stok.goalspike.com example-workspace -n operator-test

.PHONY: e2e
e2e: cli-build operator-image kind-load-image e2e-cleanup
	kubectl get ns operator-test || kubectl create ns operator-test
	kubectl apply -f deploy/crds/stok.goalspike.com_workspaces_crd.yaml -n operator-test
	kubectl apply -f deploy/crds/stok.goalspike.com_commands_crd.yaml -n operator-test
	operator-sdk test local --operator-namespace operator-test --verbose ./test/e2e/

.PHONY: e2e
e2e-cleanup:
	kubectl delete -n operator-test --all deploy
	kubectl delete -n operator-test --all command
	kubectl delete -n operator-test --all workspace
	kubectl delete -n operator-test secret secret-1 || true
	kubectl delete -n operator-test serviceaccounts stok-operator || true
	kubectl delete -n operator-test --all roles.rbac.authorization.k8s.io
	kubectl delete -n operator-test --all rolebindings.rbac.authorization.k8s.io

.PHONY: e2e-local
e2e-local: cli-build
	kubectl get ns operator-test || kubectl create ns operator-test
	kubectl apply -f deploy/crds/stok.goalspike.com_workspaces_crd.yaml -n operator-test
	kubectl apply -f deploy/crds/stok.goalspike.com_commands_crd.yaml -n operator-test
	operator-sdk test local --up-local --namespace operator-test --verbose ./test/e2e/

.PHONY: kind-load-image
kind-load-image:
	kind load docker-image leg100/stok-operator:latest

.PHONY: unit
unit: operator-unit cli-unit

.PHONY: operator-unit
operator-unit:
	go test -v ./pkg/...

.PHONY: cli-unit
cli-unit:
	go test -v ./cmd

.PHONY: crds
crds:
	kubectl apply -f deploy/crds/stok.goalspike.com_workspaces_crd.yaml
	kubectl apply -f deploy/crds/stok.goalspike.com_commands_crd.yaml

.PHONY: deploy
deploy: crds
	operator-sdk build stok-operator --image-build-args "--iidfile stok-operator.iid" && \
		TAG=$$(cat stok-operator.iid | sed 's/sha256:\(.*\)/\1/') && \
		docker tag stok-operator:latest stok-operator:$${TAG} && \
		kind load docker-image stok-operator:$${TAG} && \
		helm upgrade -i --wait --set-string image.tag=$$TAG stok-operator ./charts/stok-operator

.PHONY: operator-build
operator-build:
	go build -o stok-operator github.com/leg100/stok/cmd/manager

.PHONY: operator-image
operator-image: operator-build
	docker build -f build/Dockerfile -t leg100/stok-operator:latest .

.PHONY: generate-crds
generate-crds:
	operator-sdk generate k8s && \
	operator-sdk generate crds

.PHONY: generate-clientset
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

.PHONY: cli-build
cli-build:
	go build -o build/_output/bin/stok github.com/leg100/stok
