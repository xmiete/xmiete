# XMiete Go SDK Example

This directory contains a clean and efficient Go client example for the XMiete standard. Go is a popular choice for building scalable, cloud-native microservices in the fintech sector.

## Structure
- **Models**: Go `structs` with `json` tags that map directly to `xmiete_schema.json`.
- **Client**: A clean interface (`XMieteClient`) that utilizes `context.Context` for cancellation and timeouts.
- **Example**: `example_test.go` demonstrates basic interactions with the mock client.

## Key Features
- **Simplicity**: Easy to read, maintain, and integrate into existing Go projects.
- **Context Support**: Built-in support for request cancellation and deadlines, essential for robust financial systems.
- **Cloud-Native**: Ideal for deployment in containerized environments (Kubernetes/Docker).

## How to Run (Conceptual)
```bash
go run example_test.go
```
