# Access Control

The `install` command installs ClusterRoles (and ClusterRoleBindings) for your convenience:

* [etok-user](https://github.com/leg100/etok/blob/master/config/operator/user.yaml): includes the permissions necessary for running unprivileged commands
* [etok-admin](https://github.com/leg100/etok/blob/master/config/operator/admin.yaml): additional permissions for managing workspaces and running [privileged commands](#privileged-commands)

Amend the bindings accordingly to add/remove users. For example to amend the etok-user binding:

```bash
kubectl edit clusterrolebinding etok-user
```

Note: To restrict users to individual namespaces you'll want to create RoleBindings referencing the ClusterRoles.
