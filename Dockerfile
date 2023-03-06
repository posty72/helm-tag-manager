FROM golang:1.17-alpine

WORKDIR /app

COPY . ./

RUN go mod download

RUN go build -o /hello-world src/main.go

EXPOSE 8080

ENTRYPOINT [ "/hello-world" ]
