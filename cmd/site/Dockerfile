FROM golang:1.26.4-alpine AS builder

ARG VERSION=dev

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /site ./cmd/site

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /site /site
COPY --from=builder /app/static /static
# Fingerprinted CSS/JS (WithStaticDistDir) — served from disk, not embedded.
COPY --from=builder /app/dist /dist

EXPOSE 8080
ENTRYPOINT ["/site"]
