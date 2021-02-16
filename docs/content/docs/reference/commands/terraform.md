---
weight: 1
---

# Supported Terraform Commands

Most terraform commands are supported. A `(Q)` means it is a [queueable command]({{< ref "docs/reference/commands/_index.md" >}}).

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

{{< hint info >}}
Ensure terraform flags follow a double dash:
```bash
etok apply -- -auto-approve
```
{{< /hint  >}}
