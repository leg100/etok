NAMESPACE=operator-test

.PHONY: clean
clean:
	@echo ....... Deleting Rules and Service Account .......
	- kubectl delete -f deploy/role.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/role_binding.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/service_account.yaml -n ${NAMESPACE}
	@echo ....... Deleting Operator .......
	- kubectl delete -f deploy/operator.yaml -n ${NAMESPACE}
	@echo ....... Deleting test CRs .......
	- kubectl delete commands.terraform.goalspike.com example-command
	- kubcetl delete workspaces.terraform.goalspike.com example-workspace

.PHONY: e2e
e2e:
	kubectl apply -f deploy/crds/terraform.goalspike.com_workspaces_crd.yaml -n ${NAMESPACE}
	kubectl apply -f deploy/crds/terraform.goalspike.com_commands_crd.yaml -n ${NAMESPACE}
	operator-sdk build terraform-operator
	kind load docker-image terraform-operator:latest
	operator-sdk test local --namespace operator-test --verbose ./test/e2e/
