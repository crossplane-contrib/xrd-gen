# See https://docs.earthly.dev/docs/earthfile/features
VERSION --try --raw-output 0.8

PROJECT crossplane-contrib/xrd-gen

ARG --global GO_VERSION=1.27.2
ARG --global GOLANGCI_LINT_VERSION=v2.14.0
ARG --global GO_LICENSES_VERSION=v2.0.1
ARG --global YAMLLINT_VERSION=1.38.0
ARG --global CROSSPLANE_VERSION=2.4.1
ARG --global K3D_VERSION=5.9.0
ARG --global HELM_VERSION=3.22.0
ARG --global KUBECTL_VERSION=1.37.0

# reviewable checks that a branch is ready for review. Run it before opening a
# pull request. It will catch a lot of the things our CI workflow will catch.
reviewable:
  WAIT
    BUILD +tidy
  END
  WAIT
    BUILD +licenses
  END
  BUILD +lint
  BUILD +test

# build builds xrd-gen for your native OS and architecture.
build:
  ARG USERPLATFORM
  ARG EARTHLY_GIT_SHORT_HASH
  ARG EARTHLY_GIT_COMMIT_TIMESTAMP
  ARG XRD_GEN_VERSION=v0.0.0-${EARTHLY_GIT_COMMIT_TIMESTAMP}-${EARTHLY_GIT_SHORT_HASH}
  BUILD --platform=$USERPLATFORM +go-build --XRD_GEN_VERSION=${XRD_GEN_VERSION} --SAVE_LOCAL_BIN=true

# multiplatform-build builds xrd-gen for all supported OS and architectures.
multiplatform-build:
  ARG EARTHLY_GIT_SHORT_HASH
  ARG EARTHLY_GIT_COMMIT_TIMESTAMP
  ARG XRD_GEN_VERSION=v0.0.0-${EARTHLY_GIT_COMMIT_TIMESTAMP}-${EARTHLY_GIT_SHORT_HASH}
  ARG RELEASE_ARTIFACTS=false
  BUILD +go-multiplatform-build --RELEASE_ARTIFACTS=${RELEASE_ARTIFACTS} --XRD_GEN_VERSION=${XRD_GEN_VERSION}

# test runs unit tests and the integration tests that do not need a cluster.
test:
  BUILD +go-test

# e2e runs the integration tests that install Crossplane into a throwaway k3d
# cluster. -P (--allow-privileged) is required for Docker-in-Docker.
e2e:
  BUILD +e2e-run

# lint runs linters.
lint:
  BUILD +go-lint
  BUILD +yaml-lint

# tidy tidies and verifies go.mod and go.sum.
tidy:
  BUILD +go-modules-tidy

# licenses checks the licence of everything linked into the released binary.
licenses:
  BUILD +go-licenses-check

# go-modules downloads xrd-gen's go modules. It's the base target of most Go
# related targets (go-build, etc).
go-modules:
  ARG NATIVEPLATFORM
  FROM --platform=${NATIVEPLATFORM} golang:${GO_VERSION}
  WORKDIR /xrd-gen
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY go.mod go.sum ./
  RUN go mod download
  SAVE ARTIFACT go.mod AS LOCAL go.mod
  SAVE ARTIFACT go.sum AS LOCAL go.sum

# go-modules-tidy tidies and verifies go.mod and go.sum.
go-modules-tidy:
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir cmd/ internal/ pkg/ integrationtest/ .
  RUN go mod tidy
  RUN go mod verify
  SAVE ARTIFACT go.mod AS LOCAL go.mod
  SAVE ARTIFACT go.sum AS LOCAL go.sum

# go-build builds the xrd-gen binary for a single OS and architecture.
# Set RELEASE_ARTIFACTS=true to output flat release-ready artifacts to
# _output/release/.
go-build:
  ARG EARTHLY_GIT_SHORT_HASH
  ARG EARTHLY_GIT_COMMIT_TIMESTAMP
  ARG XRD_GEN_VERSION=v0.0.0-${EARTHLY_GIT_COMMIT_TIMESTAMP}-${EARTHLY_GIT_SHORT_HASH}
  ARG TARGETARCH
  ARG TARGETOS
  ARG GOARCH=${TARGETARCH}
  ARG GOOS=${TARGETOS}
  ARG LDFLAGS="-s -w -X=github.com/crossplane-contrib/xrd-gen/pkg/version.Version=${XRD_GEN_VERSION}"
  ARG CGO_ENABLED=0
  ARG RELEASE_ARTIFACTS=false
  ARG SAVE_LOCAL_BIN=false
  ARG SAVE_RELEASE_LICENSE=false
  FROM +go-modules
  LET ext = ""
  IF [ "$GOOS" = "windows" ]
    SET ext = ".exe"
  END
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir cmd/ internal/ pkg/ .
  COPY LICENSE .
  RUN GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=${CGO_ENABLED} go build -ldflags="${LDFLAGS}" -o xrd-gen${ext} ./cmd/xrd-gen
  RUN sha256sum xrd-gen${ext} > xrd-gen${ext}.sha256.tmp && cut -d' ' -f1 xrd-gen${ext}.sha256.tmp > xrd-gen${ext}.sha256
  RUN tar -czf xrd-gen.tar.gz xrd-gen${ext} xrd-gen${ext}.sha256 LICENSE
  RUN sha256sum xrd-gen.tar.gz > xrd-gen.tar.gz.sha256.tmp && cut -d' ' -f1 xrd-gen.tar.gz.sha256.tmp > xrd-gen.tar.gz.sha256
  IF [ "$RELEASE_ARTIFACTS" = "true" ]
    # Flat structure with arch suffix for releases: _output/release/xrd-gen_linux_amd64
    IF [ "$SAVE_RELEASE_LICENSE" = "true" ]
      SAVE ARTIFACT --keep-ts LICENSE AS LOCAL _output/release/LICENSE
    END
    SAVE ARTIFACT --keep-ts xrd-gen${ext} AS LOCAL _output/release/xrd-gen_${GOOS}_${GOARCH}${ext}
    SAVE ARTIFACT --keep-ts xrd-gen${ext}.sha256 AS LOCAL _output/release/xrd-gen_${GOOS}_${GOARCH}${ext}.sha256
    SAVE ARTIFACT --keep-ts xrd-gen.tar.gz AS LOCAL _output/release/xrd-gen_${GOOS}_${GOARCH}.tar.gz
    SAVE ARTIFACT --keep-ts xrd-gen.tar.gz.sha256 AS LOCAL _output/release/xrd-gen_${GOOS}_${GOARCH}.tar.gz.sha256
  ELSE
    # Nested structure for local development: _output/bin/linux_amd64/xrd-gen
    SAVE ARTIFACT --keep-ts xrd-gen${ext} AS LOCAL _output/bin/${GOOS}_${GOARCH}/xrd-gen${ext}
    SAVE ARTIFACT --keep-ts xrd-gen${ext}.sha256 AS LOCAL _output/bin/${GOOS}_${GOARCH}/xrd-gen${ext}.sha256
    IF [ "$SAVE_LOCAL_BIN" = "true" ]
      SAVE ARTIFACT --keep-ts xrd-gen${ext} AS LOCAL bin/xrd-gen${ext}
    END
    SAVE ARTIFACT --keep-ts xrd-gen.tar.gz AS LOCAL _output/bundle/${GOOS}_${GOARCH}/xrd-gen.tar.gz
    SAVE ARTIFACT --keep-ts xrd-gen.tar.gz.sha256 AS LOCAL _output/bundle/${GOOS}_${GOARCH}/xrd-gen.tar.gz.sha256
  END

# go-multiplatform-build builds xrd-gen binaries for all supported OS and
# architectures. GOOS/GOARCH select the Go target; the Earthly platform stays
# native.
go-multiplatform-build:
  ARG NATIVEPLATFORM
  ARG XRD_GEN_VERSION
  ARG RELEASE_ARTIFACTS=false
  BUILD --platform=$NATIVEPLATFORM +go-build --GOOS=linux --GOARCH=amd64 --RELEASE_ARTIFACTS=${RELEASE_ARTIFACTS} --XRD_GEN_VERSION=${XRD_GEN_VERSION} --SAVE_RELEASE_LICENSE=true
  BUILD --platform=$NATIVEPLATFORM +go-build --GOOS=linux --GOARCH=arm64 --RELEASE_ARTIFACTS=${RELEASE_ARTIFACTS} --XRD_GEN_VERSION=${XRD_GEN_VERSION}
  BUILD --platform=$NATIVEPLATFORM +go-build --GOOS=linux --GOARCH=arm --RELEASE_ARTIFACTS=${RELEASE_ARTIFACTS} --XRD_GEN_VERSION=${XRD_GEN_VERSION}
  BUILD --platform=$NATIVEPLATFORM +go-build --GOOS=linux --GOARCH=ppc64le --RELEASE_ARTIFACTS=${RELEASE_ARTIFACTS} --XRD_GEN_VERSION=${XRD_GEN_VERSION}
  BUILD --platform=$NATIVEPLATFORM +go-build --GOOS=darwin --GOARCH=amd64 --RELEASE_ARTIFACTS=${RELEASE_ARTIFACTS} --XRD_GEN_VERSION=${XRD_GEN_VERSION}
  BUILD --platform=$NATIVEPLATFORM +go-build --GOOS=darwin --GOARCH=arm64 --RELEASE_ARTIFACTS=${RELEASE_ARTIFACTS} --XRD_GEN_VERSION=${XRD_GEN_VERSION}
  BUILD --platform=$NATIVEPLATFORM +go-build --GOOS=windows --GOARCH=amd64 --RELEASE_ARTIFACTS=${RELEASE_ARTIFACTS} --XRD_GEN_VERSION=${XRD_GEN_VERSION}

# go-test runs Go unit tests plus the integration tests that only need the
# xrd-gen binary. -short skips the ones that stand up a Kubernetes cluster.
go-test:
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir cmd/ internal/ pkg/ integrationtest/ .
  RUN mkdir -p bin && go build -o bin/xrd-gen ./cmd/xrd-gen
  RUN go test -short -race -covermode=atomic -coverprofile=coverage.txt ./...
  SAVE ARTIFACT coverage.txt AS LOCAL _output/tests/coverage.txt

# go-lint lints Go code.
go-lint:
  FROM +go-modules
  # This cache is private because golangci-lint doesn't support concurrent runs.
  CACHE --id go-lint --sharing private /root/.cache/golangci-lint
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY +golangci-lint-setup/golangci-lint /usr/local/bin/golangci-lint
  COPY .golangci.yaml .
  COPY --dir cmd/ internal/ pkg/ integrationtest/ .
  RUN golangci-lint run

# golangci-lint-setup takes the golangci-lint binary from its published image,
# so +go-lint can run it on the same Go toolchain the rest of the build uses.
golangci-lint-setup:
  ARG NATIVEPLATFORM
  FROM --platform=${NATIVEPLATFORM} golangci/golangci-lint:${GOLANGCI_LINT_VERSION}
  SAVE ARTIFACT /usr/bin/golangci-lint golangci-lint

# go-licenses-check fails the build if anything linked into the released binary
# carries a licence outside ALLOWED_LICENSES. It reports per package rather than
# per module, so it also covers third-party code vendored inside a dependency
# under a different licence from its parent.
#
# This runs in a container on purpose: go-licenses identifies the standard
# library by comparing against the GOROOT it was built against, and fails
# outright when the go.mod toolchain is newer than the local Go. Here GO_VERSION
# and the toolchain agree.
go-licenses-check:
  ARG ALLOWED_LICENSES=Apache-2.0,BSD-2-Clause,BSD-3-Clause,ISC,MIT
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY LICENSE .
  COPY --dir cmd/ internal/ pkg/ .
  RUN go install github.com/google/go-licenses/v2@${GO_LICENSES_VERSION}
  RUN go-licenses check ./cmd/... --allowed_licenses=${ALLOWED_LICENSES}

# yaml-lint lints YAML files.
yaml-lint:
  FROM python:3-alpine
  RUN pip install --no-cache-dir yamllint==${YAMLLINT_VERSION}
  WORKDIR /xrd-gen
  COPY .yamllint .
  COPY --dir .github/ .
  RUN yamllint -c .yamllint . --no-warnings

# e2e-artifacts compiles the xrd-gen binary and the integration test binary, so
# that +e2e-run needs a Docker daemon but not a Go toolchain. Only the
# TestCrossplane* tests can run without one; the rest shell out to go build to
# check that generated code compiles, and run in +go-test.
e2e-artifacts:
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir cmd/ internal/ pkg/ integrationtest/ .
  RUN go build -o xrd-gen ./cmd/xrd-gen
  RUN go test -c -o integrationtest.test ./integrationtest
  SAVE ARTIFACT xrd-gen
  SAVE ARTIFACT integrationtest.test

# e2e-run installs Crossplane into a throwaway k3d cluster and applies XRDs
# generated by xrd-gen to it.
e2e-run:
  FROM earthly/dind:ubuntu-24.04-docker-27.3.1-1@sha256:b15b25ca16da19d72da9aa5ad7dea511451e01e75a6d3e06eface760dde3d3cc
  RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
  RUN curl -fsSL -o /tmp/k3d-linux-amd64 \
    "https://github.com/k3d-io/k3d/releases/download/v${K3D_VERSION}/k3d-linux-amd64" \
    && curl -fsSL -o /tmp/k3d-checksums.txt \
    "https://github.com/k3d-io/k3d/releases/download/v${K3D_VERSION}/checksums.txt" \
    && awk '/k3d-linux-amd64$/ {print $1"  k3d-linux-amd64"}' /tmp/k3d-checksums.txt > /tmp/k3d.sha256 \
    && grep -Eq '^[0-9a-f]{64}  k3d-linux-amd64$' /tmp/k3d.sha256 \
    && [ "$(grep -c . /tmp/k3d.sha256)" -eq 1 ] \
    && (cd /tmp && sha256sum -c k3d.sha256) \
    && install -m 0755 /tmp/k3d-linux-amd64 /usr/local/bin/k3d
  RUN curl -fsSL -o /tmp/helm-v${HELM_VERSION}-linux-amd64.tar.gz \
    "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" \
    && curl -fsSL -o /tmp/helm.sha256sum \
    "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz.sha256sum" \
    && grep -Eq '^[0-9a-f]{64}  helm-v'"${HELM_VERSION}"'-linux-amd64.tar.gz$' /tmp/helm.sha256sum \
    && [ "$(grep -c . /tmp/helm.sha256sum)" -eq 1 ] \
    && (cd /tmp && sha256sum -c helm.sha256sum) \
    && tar -xz -C /tmp -f /tmp/helm-v${HELM_VERSION}-linux-amd64.tar.gz \
    && install -m 0755 /tmp/linux-amd64/helm /usr/local/bin/helm
  RUN curl -fsSL -o /tmp/kubectl \
    "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    && curl -fsSL -o /tmp/kubectl.sha256 \
    "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256" \
    && grep -Eq '^[0-9a-f]{64}$' /tmp/kubectl.sha256 \
    && echo "$(cat /tmp/kubectl.sha256)  /tmp/kubectl" | sha256sum -c - \
    && install -m 0755 /tmp/kubectl /usr/local/bin/kubectl
  WORKDIR /xrd-gen
  COPY +e2e-artifacts/xrd-gen bin/xrd-gen
  COPY +e2e-artifacts/integrationtest.test integrationtest/
  COPY --dir integrationtest/testdata integrationtest/
  WORKDIR /xrd-gen/integrationtest
  ENV CROSSPLANE_VERSION=${CROSSPLANE_VERSION}
  WITH DOCKER
    RUN ./integrationtest.test -test.v -test.timeout 30m -test.run 'TestCrossplane'
  END
