.PHONY: clean
clean:
	@echo ....... Deleting Rules and Service Account .......
	- kubectl delete -f operator/deploy/role.yaml -n operator-test
	- kubectl delete -f operator/deploy/role_binding.yaml -n operator-test
	- kubectl delete -f operator/deploy/service_account.yaml -n operator-test
	@echo ....... Deleting Operator .......
	- kubectl delete -f operator/deploy/operator.yaml -n operator-test
	@echo ....... Deleting test CRs .......
	- kubectl delete commands.terraform.goalspike.com example-command -n operator-test
	- kubcetl delete workspaces.terraform.goalspike.com example-workspace -n operator-test

.PHONY: e2e
e2e: operator-image
	kubectl get ns operator-test || kubectl create ns operator-test
	kubectl apply -f operator/deploy/crds/terraform.goalspike.com_workspaces_crd.yaml -n operator-test
	kubectl apply -f operator/deploy/crds/terraform.goalspike.com_commands_crd.yaml -n operator-test
	kind load docker-image leg100/stok-operator:latest
	cd operator && operator-sdk test local --namespace operator-test --verbose ./test/e2e/

.PHONY: unit
unit:
	go test ./operator/pkg/...

.PHONY: crds
crds:
	kubectl apply -f operator/deploy/crds/terraform.goalspike.com_workspaces_crd.yaml
	kubectl apply -f operator/deploy/crds/terraform.goalspike.com_commands_crd.yaml

.PHONY: deploy
deploy: crds
	operator-sdk build stok-operator --image-build-args "--iidfile stok-operator.iid" && \
		TAG=$$(cat stok-operator.iid | sed 's/sha256:\(.*\)/\1/') && \
		docker tag stok-operator:latest stok-operator:$${TAG} && \
		kind load docker-image stok-operator:$${TAG} && \
		helm upgrade -i --wait --set-string image.tag=$$TAG stok-operator ./charts/stok-operator

operator-build:
	go build -o operator/build/_output/bin/stok-operator \
		github.com/leg100/stok/operator/cmd/manager

operator-image: operator-build
	docker build -f operator/build/Dockerfile -t leg100/stok-operator:latest operator/
