FROM alpine:3.12

ENV STOK_BIN=/usr/local/bin/stok \
    USER_UID=1001 \
    USER_NAME=stok

ENV TERRAFORM_VERSION=0.12.27

# install terraform
RUN apk add curl && \
    curl -LOs https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    curl -LOs https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_SHA256SUMS && \
    sed -n "/terraform_${TERRAFORM_VERSION}_linux_amd64.zip/p" terraform_${TERRAFORM_VERSION}_SHA256SUMS | sha256sum -c && \
    unzip terraform_${TERRAFORM_VERSION}_linux_amd64.zip -d /usr/local/bin/ && \
    rm terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    rm terraform_${TERRAFORM_VERSION}_SHA256SUMS

# install operator binary
COPY build/_output/bin/stok ${STOK_BIN}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
