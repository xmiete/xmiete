# XMiete Core

The open-source standard for digital rental deposits.

## Overview
XMiete Core provides a unified JSON schema and API design to digitize the entire lifecycle of a rental deposit—from application and identification to pledging and release.

## Features
- **Modular Schema:** Supports `CASH_EQUIVALENT`, `BANK_GUARANTEE`, and `INSURANCE`.
- **eID Integration:** Built-in fields for eID verification status.
- **Tax Compliance:** Automated Tax ID (Steuer-ID) validation logic support.
- **Legal Ready:** Designed to meet BGB § 551 requirements.

## Stakeholders
- **Fintechs & Brands:** heykaution, getmomo, PlusForta, smartmiete
- **Banks & Partners:** 
  - Aareal Bank (GetMomo)
  - Volksbank (PlusForta partnership)
  - Instabank (Smartmiete partnership)
  - Hausbank München eG
  - DKB (Deutsche Kreditbank)
  - Sparkassen-Finanzgruppe
  - PSD Banken
- **Insurances:** Support for Mietkautionsbürgschaften (e.g., Kautionsfrei)

## Getting Started
The core of this project is the `xmiete_schema.json`. See the documentation for implementation details.

## License
XMiete Core is dual-licensed to ensure both widespread adoption of the standard and legal protection for the code:

- **Specification & Documentation:** Licensed under [Creative Commons Attribution 4.0 International (CC BY 4.0)](LICENSE-SPECIFICATION). This covers the JSON schemas (`.json`), API definitions (`.yaml`), and all Markdown documentation.
- **Code & SDK Examples:** Licensed under the [Apache License, Version 2.0](LICENSE). This covers all source code in the `sdk-examples/`, `tests/`, and helper scripts.

Copyright © 2026 XMiete Core Contributors
