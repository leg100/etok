FROM alpine:3.12

ENV STOK_BIN=/usr/local/bin/stok

# Any change to this path must also be made to the constant terraformBinMountPath in /pkg/runner/runner.go
ENV TF_BIN_PATH=/terraform-bins

ENV PATH=${TF_BIN_PATH}:${PATH}

ARG TERRAFORM_VERSION=0.13.5

# install terraform
RUN apk add curl && \
    curl -LOs https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    curl -LOs https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_SHA256SUMS && \
    sed -n "/terraform_${TERRAFORM_VERSION}_linux_amd64.zip/p" terraform_${TERRAFORM_VERSION}_SHA256SUMS | sha256sum -c && \
    mkdir -p ${TF_BIN_PATH} && \
    unzip terraform_${TERRAFORM_VERSION}_linux_amd64.zip -d ${TF_BIN_PATH} && \
    rm terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    rm terraform_${TERRAFORM_VERSION}_SHA256SUMS

# Stok binary is expected to be copied from the PWD because that is where goreleaser builds it and
# there does not appear to be a way to customise a different location
COPY stok ${STOK_BIN}

# Copy over adjunct scripts, i.e. entrypoint and user_setup
COPY build/bin /usr/local/bin

ENTRYPOINT ["/usr/local/bin/entrypoint"]
