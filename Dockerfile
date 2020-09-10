FROM golang:latest as builder
WORKDIR /go/src/app
COPY main.go .
RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM alpine:latest
ENV PORT 8080
EXPOSE 8080
WORKDIR /root/
COPY --from=0 /go/src/app .
COPY index.html . 
CMD ["./app"]