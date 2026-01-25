FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o ./out/server ./cmd/server

EXPOSE 8088

CMD [ "/app/out/server" ]
