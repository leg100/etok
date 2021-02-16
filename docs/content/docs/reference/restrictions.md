# Restrictions

Both the terraform configuration and the terraform state, after compression, are subject to a 1MiB limit. This due to the fact that they are stored in a config map and a secret respectively, and the data stored in either cannot exceed 1MiB.
