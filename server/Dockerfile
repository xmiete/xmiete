FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /xmiete-server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=builder /xmiete-server /xmiete-server
EXPOSE 8080
ENTRYPOINT ["/xmiete-server"]
