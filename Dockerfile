FROM golang:latest AS builder

WORKDIR /app

COPY . .

RUN rm go.mod go.sum
RUN go mod init main
RUN go get -u
RUN CGO_ENABLED=0 go build main.go

FROM ubuntu:latest AS final

RUN apt update -y && apt upgrade -y && apt install -y ca-certificates
COPY --from=builder /app/ /app/

WORKDIR /app

EXPOSE 80/tcp 443/tcp

VOLUME ["/app/certs"]

ENTRYPOINT [ "./main" ]