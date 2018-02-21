FROM golang:1.9-alpine3.7 as build

WORKDIR /go/src/github.com/richiefi/imageproxy
ADD . .

RUN apk add --update autoconf automake build-base libtool nasm git
RUN ./build-mozjpeg.sh
RUN go install github.com/richiefi/imageproxy/cmd/imageproxy

FROM alpine:3.7
RUN apk add --update ca-certificates

WORKDIR /go/bin
COPY --from=build /usr/local/lib64/ /usr/local/lib64/
COPY --from=build /go/bin/imageproxy .
COPY --from=build /go/src/github.com/richiefi/imageproxy/entrypoint.sh .

ENV PORT 8080
EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
