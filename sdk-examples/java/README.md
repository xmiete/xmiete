# XMiete Java SDK Example

This directory contains a robust, type-safe Java client example for the XMiete standard. Java is the primary language used in German banking systems (Sparkassen, Volksbanken, etc.), making this a key integration path.

## Structure
- **Models**: Java 17 `records` that map directly to `xmiete_schema.json`.
- **Client**: An asynchronous interface (`XMieteClient`) using `CompletableFuture` for non-blocking API calls.
- **Example**: `ExampleUsage` shows how a bank would confirm a pledge or a manager would retrieve a deposit status.

## Key Features
- **Type Safety**: Prevents malformed requests at compile-time.
- **Asynchrony**: Designed for high-performance enterprise applications.
- **BGB § 551 Ready**: Built-in support for legal pledge confirmation flows.

## Integration
In a production environment, this SDK would typically be packaged as a JAR and integrated using Spring Boot (Feign) or the standard Java 11+ `HttpClient`.
