FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags "-s -w" -o /out/prism ./cmd/prism

FROM alpine:3.22

RUN apk add --no-cache ca-certificates && \
    adduser -D -u 10001 prism

COPY --from=build /out/prism /usr/local/bin/prism

USER prism
ENTRYPOINT ["/usr/local/bin/prism"]
CMD ["--help"]
