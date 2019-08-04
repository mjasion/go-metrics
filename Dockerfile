FROM golang:alpine AS build-env
RUN apk add --update --no-cache git
RUN go get -u github.com/golang/dep/cmd/dep
ADD . /go/src/go-metrics
WORKDIR /go/src/go-metrics
RUN dep ensure && go build -o go-metrics

# final stage
FROM alpine:latest
RUN apk add --update --no-cache curl
COPY --from=build-env /go/src/go-metrics/go-metrics /go-metrics
EXPOSE 8080
CMD ["/go-metrics"]
