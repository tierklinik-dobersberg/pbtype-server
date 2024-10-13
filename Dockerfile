
# Build the gobinary

FROM golang:1.23 AS gobuild

RUN update-ca-certificates

WORKDIR /go/src/app

COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download
RUN go mod verify

COPY ./ ./

RUN CGO_ENABLED=0 go build -o /go/bin/pbtype-server ./cmds/pbtype-server

FROM gcr.io/distroless/base-debian12

COPY --from=gobuild /go/bin/pbtype-server /go/bin/pbtype-server
EXPOSE 8081

ENTRYPOINT ["/go/bin/pbtype-server"]
