FROM golang:1.9-alpine3.7 as build

WORKDIR /go/src/github.com/richiefi/imageproxy
ADD . .

WORKDIR /go/src/github.com/richiefi/imageproxy/cmd/imageproxy
RUN go-wrapper download
RUN CGO_ENABLED=0 GOOS=linux go-wrapper install

FROM alpine:3.7

WORKDIR /go/bin

RUN apk add --update ca-certificates

COPY --from=build /go/bin/imageproxy .
CMD ["-addr", "0.0.0.0:8080"]
ENTRYPOINT ["/go/bin/imageproxy"]

EXPOSE 8080
