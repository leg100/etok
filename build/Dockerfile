FROM alpine:3.12

ENV ETOK_BIN=/usr/local/bin/etok

# Any change to this path must also be made to the constant binMountPath in /pkg/controllers/paths.go
ENV TF_BIN_PATH=/terraform-bins

ENV PATH=${TF_BIN_PATH}:${PATH}

# Any change to this version must also be made to the kubebuilder default marker for TerraformVersion
# (in /api/etok.dev/v1alpha1/workspace_types.go)
ARG TERRAFORM_VERSION=0.14.3

# Install terraform, as well as git for github webhook, and jq is used by `etok
# workspace new` to parse the terraform version
RUN apk add curl jq git && \
    curl -LOs https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    curl -LOs https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_SHA256SUMS && \
    sed -n "/terraform_${TERRAFORM_VERSION}_linux_amd64.zip/p" terraform_${TERRAFORM_VERSION}_SHA256SUMS | sha256sum -c && \
    unzip terraform_${TERRAFORM_VERSION}_linux_amd64.zip -d /usr/local/bin && \
    rm terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    rm terraform_${TERRAFORM_VERSION}_SHA256SUMS

# Etok binary is expected to be copied from the PWD because that is where goreleaser builds it and
# there does not appear to be a way to customise a different location
COPY etok ${ETOK_BIN}

# Copy over adjunct scripts, i.e. entrypoint and user_setup
COPY build/bin /usr/local/bin

ENTRYPOINT ["/usr/local/bin/entrypoint"]
