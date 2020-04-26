ARG TERRAFORM_VERSION=0.12.21
ARG KUBECTL_VERSION=1.17.0
FROM hashicorp/terraform:${TERRAFORM_VERSION}

RUN apk update \
        && apk --no-cache add jq curl \
        && curl -L https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl \
        -o /bin/kubectl \
        && chmod +x /bin/kubectl \
        && apk del curl
