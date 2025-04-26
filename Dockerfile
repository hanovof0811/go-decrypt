FROM golang:1.21-alpine

RUN apk add --no-cache bash curl bento4

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o decrypt-service main.go

EXPOSE 9900
CMD ["./decrypt-service"]
