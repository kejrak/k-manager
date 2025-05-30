FROM golang:1.23.5-alpine

WORKDIR /app


COPY go.mod go.sum ./


RUN go mod download


COPY . .


RUN CGO_ENABLED=0 GOOS=linux go build -o main .


FROM alpine:3.19

WORKDIR /app


COPY --from=0 /app/main .


EXPOSE 8080


CMD ["./main"] 