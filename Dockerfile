FROM golang:bullseye as builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . . 
RUN CGO_ENABLED=0 go build -a -o cncstats main.go

FROM alpine:3.15
RUN apk add --no-cache git
COPY --from=builder /build/cncstats /usr/bin/cncstats
COPY --from=builder /build/inizh /var
#ENTRYPOINT ["ls", "/var/Data/INI/Object"]
ENTRYPOINT ["/usr/bin/cncstats"]
