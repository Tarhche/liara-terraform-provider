ARG OPENAPI_SPEC_VERSION

FROM golang:1.24-alpine

# Install required tools
RUN apk update && apk add --no-cache gnupg git ca-certificates wget unzip

# install Terraform
ARG PRODUCT=terraform
ARG VERSION=1.12.2

RUN cd /tmp && \
    ARCH=$(uname -m) && \
    case ${ARCH} in \
        x86_64) ARCH="amd64" ;; \
        aarch64) ARCH="arm64" ;; \
        *) echo "Unsupported architecture: ${ARCH}" && exit 1 ;; \
    esac && \
    wget https://releases.hashicorp.com/${PRODUCT}/${VERSION}/${PRODUCT}_${VERSION}_linux_${ARCH}.zip \
    && wget https://releases.hashicorp.com/${PRODUCT}/${VERSION}/${PRODUCT}_${VERSION}_SHA256SUMS \
    && wget https://releases.hashicorp.com/${PRODUCT}/${VERSION}/${PRODUCT}_${VERSION}_SHA256SUMS.sig \
    && wget -qO- https://www.hashicorp.com/.well-known/pgp-key.txt | gpg --import \
    && gpg --verify ${PRODUCT}_${VERSION}_SHA256SUMS.sig ${PRODUCT}_${VERSION}_SHA256SUMS \
    && grep ${PRODUCT}_${VERSION}_linux_${ARCH}.zip ${PRODUCT}_${VERSION}_SHA256SUMS | sha256sum -c \
    && unzip -qo /tmp/${PRODUCT}_${VERSION}_linux_${ARCH}.zip -d /tmp \
    && mv /tmp/${PRODUCT} /usr/local/bin/${PRODUCT} \
    && rm -f /tmp/${PRODUCT}_${VERSION}_linux_${ARCH}.zip ${PRODUCT}_${VERSION}_SHA256SUMS ${VERSION}/${PRODUCT}_${VERSION}_SHA256SUMS.sig \
    && rm -rf /tmp/*

# Clone the Liara OpenAPI repository and checkout the specific version
RUN git clone --depth 1 --single-branch --branch main https://github.com/liara-cloud/openapi.git /tmp/liara-openapi \
    && cd /tmp/liara-openapi \
    && git fetch --depth 1 origin ${OPENAPI_SPEC_VERSION} \
    && git checkout ${OPENAPI_SPEC_VERSION}

WORKDIR /home

# Download dependencies and clean up the working directory again
COPY go.mod go.sum ./
RUN go mod download && rm -rf go.mod go.sum
