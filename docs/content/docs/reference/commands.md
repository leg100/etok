# Commands

## Supported Terraform Commands

* `apply`(Q)
* `console`
* `destroy`(Q)
* `fmt`
* `force-unlock`(Q)
* `get`
* `graph`
* `import`(Q)
* `init`(Q)
* `output`
* `plan`
* `providers`
* `providers lock`
* `refresh`(Q)
* `state list`
* `state mv`(Q)
* `state pull`
* `state push`(Q)
* `state replace-provider`(Q)
* `state rm`(Q)
* `state show`
* `show`
* `taint`(Q)
* `untaint`(Q)
* `validate`

## Additional Commands

* `sh`(Q) - run shell or arbitrary command in workspace

## Privileged Commands

Commands can be specified as privileged. Only users possessing the RBAC permission to update the workspace (see below) can run privileged commands. Specify them via the `--privileged-commands` flag when creating a new workspace with `workspace new`.

## Queueable Commands (Q)

Commands with the ability to alter state are deemed 'queueable': only one queueable command at a time can run on a workspace. The currently running command is designated as 'active', and commands waiting to become active wait in a workspace FIFO queue.

All other commands run immediately and concurrently.
