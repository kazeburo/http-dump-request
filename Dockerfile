FROM golang:1.15.7-buster as builder
RUN apt-get update
WORKDIR /go/src/app
COPY . .
RUN go get github.com/rakyll/statik
RUN CGO_ENABLED=0 GOOS=linux make

FROM alpine:latest
EXPOSE 3000
COPY --from=builder /go/src/app/http-dump-request /http-dump-request
ENTRYPOINT ["/http-dump-request"]