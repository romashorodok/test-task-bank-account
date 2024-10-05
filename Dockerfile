FROM golang:1.23.2-alpine3.20 as account-builder
WORKDIR /app
RUN go env -w GOCACHE=/go-cache

ARG MOD=account

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/go-cache \
    --mount=type=bind,source=.,target=.,readonly \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /go/bin/${MOD} ./${MOD}/cmd/main.go


# CMD [ "sh", "-c", /go/bin/${MOD} ]

