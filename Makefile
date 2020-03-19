.PHONY: clean
clean:
	@echo ....... Deleting Rules and Service Account .......
	- kubectl delete -f deploy/role.yaml -n operator-test
	- kubectl delete -f deploy/role_binding.yaml -n operator-test
	- kubectl delete -f deploy/service_account.yaml -n operator-test
	@echo ....... Deleting Operator .......
	- kubectl delete -f deploy/operator.yaml -n operator-test
	@echo ....... Deleting test CRs .......
	- kubectl delete commands.terraform.goalspike.com example-command -n operator-test
	- kubcetl delete workspaces.terraform.goalspike.com example-workspace -n operator-test

.PHONY: e2e
e2e:
	kubectl get ns operator-test || kubectl create ns operator-test
	kubectl apply -f deploy/crds/terraform.goalspike.com_workspaces_crd.yaml -n operator-test
	kubectl apply -f deploy/crds/terraform.goalspike.com_commands_crd.yaml -n operator-test
	operator-sdk build terraform-operator
	kind load docker-image terraform-operator:latest
	operator-sdk test local --namespace operator-test --verbose ./test/e2e/

.PHONY: unit
unit:
	go test ./pkg/...

.PHONY: crds
crds:
	kubectl apply -f deploy/crds/terraform.goalspike.com_workspaces_crd.yaml
	kubectl apply -f deploy/crds/terraform.goalspike.com_commands_crd.yaml

.PHONY: deploy
deploy: crds
	operator-sdk build terraform-operator --image-build-args "--iidfile terraform-operator.iid" && \
		TAG=$$(cat terraform-operator.iid | sed 's/sha256:\(.*\)/\1/') && \
		docker tag terraform-operator:latest terraform-operator:$${TAG} && \
		kind load docker-image terraform-operator:$${TAG} && \
		helm upgrade -i --wait --set-string image.tag=$$TAG terraform-operator ./charts/terraform-operator
