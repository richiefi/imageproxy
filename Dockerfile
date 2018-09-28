FROM golang:1-alpine AS build

WORKDIR /go/src/github.com/richiefi/imageproxy
ADD . .

RUN go install -ldflags="-X github.com/richiefi/imageproxy.buildVersion=`date +%s`" github.com/richiefi/imageproxy/cmd/imageproxy

FROM alpine:3.7
RUN apk add --update ca-certificates

WORKDIR /go/bin
COPY --from=build /go/bin/imageproxy .
COPY --from=build /go/src/github.com/richiefi/imageproxy/entrypoint.sh .

ENV PORT 8080
EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
