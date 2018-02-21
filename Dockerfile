FROM golang:1.9-alpine3.7 as build

WORKDIR /go/src/github.com/richiefi/imageproxy
ADD . .

RUN apk add --update libjpeg-turbo-dev automake build-base libtool nasm
RUN go install github.com/richiefi/imageproxy/cmd/imageproxy

FROM alpine:3.7

WORKDIR /go/bin

RUN apk add --update ca-certificates
RUN apk add libjpeg-turbo

COPY --from=build /go/bin/imageproxy .
COPY --from=build /go/src/github.com/richiefi/imageproxy/entrypoint.sh .

ENV PORT 8080
EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
