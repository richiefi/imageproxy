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
COPY --from=build /go/src/github.com/richiefi/imageproxy/entrypoint.sh .

ENV PORT 8080
EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
