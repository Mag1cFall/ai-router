FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12 AS runner

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY config.example.yaml /app/config.yaml

EXPOSE 8446

USER nonroot:nonroot

ENTRYPOINT ["/app/server"]
