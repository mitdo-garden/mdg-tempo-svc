FROM golang:1.27.4-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/mdg-tempo-svc ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/mdg-tempo-svc ./mdg-tempo-svc
COPY configs ./configs
RUN mkdir -p /app/uploads
EXPOSE 8081
ENTRYPOINT ["./mdg-tempo-svc"]
