FROM golang:1.27 AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

FROM alpine:3.24

COPY --from=build /api /usr/local/bin/api

ENTRYPOINT ["api"]