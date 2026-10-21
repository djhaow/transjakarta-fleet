# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.25-alpine AS build

WORKDIR /src

# Cache dependencies in their own layer.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .

# Build all three commands from the same source tree.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mock-publisher ./cmd/mock-publisher

# ---- Runtime stage ----
FROM alpine:3.20

RUN addgroup -S fleet && adduser -S -G fleet fleet

COPY --from=build /out/ /usr/local/bin/

USER fleet
CMD ["server"]
