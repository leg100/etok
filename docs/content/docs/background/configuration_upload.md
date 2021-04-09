# What is uploaded to the pod when running a command?

The contents of the root module (the current working directory, or the value of the `path` flag) is uploaded. Additionally, if the root module configuration contains references to other modules on the local filesystem, then these too are uploaded, along with all such modules recursively referenced (modules referencing modules, and so forth). The directory structure of your git repository is maintained on the kubernetes pod, ensuring relative references remain valid (e.g. `./modules/vpc` or `../modules/vpc`).

Etok supports the use of a [`.terraformignore`](https://www.terraform.io/docs/backends/types/remote.html#excluding-files-from-upload-with-terraformignore) file. Etok expects to find the file at the root of your git repository. If not found then the default set of rules apply:

* `.git/` directories
* `.terraform/` directories, exclusive of `.terraform/modules`
