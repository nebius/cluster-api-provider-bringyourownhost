# Build the manager binary
ARG DOCKER_REGISTRY="docker.io"
ARG GCR_REGISTRY="gcr.io"
FROM ${DOCKER_REGISTRY}/golang:1.20.7 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
# RUN go env -w GOPROXY=https://goproxy.cn,direct
ARG GOPROXY="https://proxy.golang.org/cached-only"
ENV GOPROXY=${GOPROXY}
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY installer/ installer/
COPY common/ common/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM ${GCR_REGISTRY}/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
