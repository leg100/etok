# Commands

Most terraform commands are [supported]({{< ref "terraform.md" >}}).

There are some useful [additional commands]({{< ref "additional.md" >}}) as well.

## Privileged Commands

Commands can be specified as privileged. Only users possessing the RBAC permission to update the workspace (see below) can run privileged commands. Specify them via the `--privileged-commands` flag when creating a new workspace with `workspace new`.

## Queueable Commands (Q)

Commands with the ability to alter state are deemed 'queueable': only one queueable command at a time can run on a workspace. The currently running command is designated as 'active', and commands waiting to become active wait in a workspace FIFO queue.

All other commands run immediately and concurrently.
