FROM golang:1.24-bookworm AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -a -o cncstats main.go

FROM alpine:3.20
COPY --from=builder /build/cncstats /usr/bin/cncstats
COPY --from=builder /build/inizh /var
COPY --from=builder /build/docs/openapi3.json /docs/openapi3.json
COPY --from=builder /build/docs/openapi3.yaml /docs/openapi3.yaml
WORKDIR /
ENTRYPOINT ["/usr/bin/cncstats"]
