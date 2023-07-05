FROM golang as builder
LABEL org.opencontainers.image.source="https://github.com/xruins/docker_state_exporter"
COPY *.go $GOPATH/src/mypackage/myapp/
COPY go.* $GOPATH/src/mypackage/myapp/
WORKDIR $GOPATH/src/mypackage/myapp/
RUN go mod init && go mod tidy
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /go/bin/docker_state_exporter

FROM gcr.io/distroless/base-debian11
COPY --from=builder /go/bin/docker_state_exporter /go/bin/docker_state_exporter
EXPOSE 8080
ENTRYPOINT ["/go/bin/docker_state_exporter"]
CMD ["-listen-address=:8080"]
