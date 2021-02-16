# Access Control

The `install` command installs ClusterRoles (and ClusterRoleBindings) for your convenience:

* [etok-user](./config/rbac/user.yaml): includes the permissions necessary for running unprivileged commands
* [etok-admin](./config/rbac/admin.yaml): additional permissions for managing workspaces and running [privileged commands](#privileged-commands)

Amend the bindings accordingly to add/remove users. For example to amend the etok-user binding:

```bash
kubectl edit clusterrolebinding etok-user
```

Note: To restrict users to individual namespaces you'll want to create RoleBindings referencing the ClusterRoles.
