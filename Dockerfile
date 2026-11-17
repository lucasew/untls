FROM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build-env

WORKDIR /go/src/untls

# COPY go.mod go.sum ./
COPY go.mod ./

RUN go mod download

COPY . ./

ARG VERSION_LONG
ENV VERSION_LONG=$VERSION_LONG

ARG VERSION_GIT
ENV VERSION_GIT=$VERSION_GIT

RUN go build -v -o untls .

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

COPY --from=build-env /go/src/untls/untls /usr/local/bin

ENTRYPOINT [ "/usr/local/bin/untls" ]

