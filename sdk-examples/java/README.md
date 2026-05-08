# XMiete Java SDK Example

A robust, type-safe Java client example for the XMiete standard. Java is the primary language used in German banking systems (Sparkassen, Volksbanken, etc.), making this a key integration path for bank partners.

## Structure

- **`models/`** — Java 17 records that map directly to `xmiete_schema.json`.
- **`client/`** — Asynchronous `XMieteClient` interface using `CompletableFuture` for non-blocking API calls.
- **`ExampleUsage`** — Shows how a bank confirms a pledge and how a property manager retrieves deposit status.

## Key Features

- **Type safety** — Compile-time validation prevents malformed requests.
- **Asynchronous** — Designed for high-performance enterprise applications.
- **eIDAS 2.0 ready** — `WalletMetadata` model covers PID, EAA, QEAA, and MDL credential types.
- **BGB § 551 compliant** — Built-in support for legal pledge confirmation flows.

## Roadmap

- **QEAA Issuance Flow client** — Implement the bank-side OID4VCI Pre-Authorized Code Flow in Java: issuance session handling, SD-JWT `KautionsPfandNachweis` credential construction (using `nimbus-jose-jwt`), token endpoint client, and credential status polling. Priority language given Java's dominance in German banking infrastructure.

## Integration

In a production environment, this SDK is typically packaged as a JAR and integrated via Spring Boot (Feign) or the standard Java 11+ `HttpClient`.
