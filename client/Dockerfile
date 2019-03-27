FROM golang:1.12.1 AS build
COPY . /go/src/app

WORKDIR /go/src/app
RUN go get -d ./... && \
 CGO_ENABLED=0 go build -a -ldflags '-s' -installsuffix cgo -o app .

# copy the binary from the build stage to the final stage
FROM alpine:3.8
COPY --from=build /go/src/app/app /app
CMD ["/app"]