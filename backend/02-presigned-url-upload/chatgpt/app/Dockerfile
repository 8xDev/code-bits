FROM golang:1.21-alpine AS build

WORKDIR /src
RUN apk add --no-cache git build-base ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /posts-service ./cmd

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /posts-service /posts-service
WORKDIR /
EXPOSE 8080
CMD ["/posts-service"]
