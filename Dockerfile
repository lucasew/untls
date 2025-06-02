FROM golang:1.24-alpine AS build-env

WORKDIR /go/src/untls

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

ARG VERSION_LONG
ENV VERSION_LONG=$VERSION_LONG

ARG VERSION_GIT
ENV VERSION_GIT=$VERSION_GIT

RUN go build -v -o untls .

FROM alpine:3.21

COPY --from=build-env /go/src/untls/untls /usr/local/bin

ENTRYPOINT [ "/usr/local/bin/untls" ]

