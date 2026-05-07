# XMiete Core - Project Instructions

## Project Scope
XMiete Core is an open-source standard (JSON/REST) for the digital rental deposit (Mietkaution). It bridges the gap between Fintechs, Banks, Insurance companies, and Property Management software.

### Key Stakeholders
- **Fintechs/Brands:** heykaution, getmomo, smartmiete, PlusForta
- **Banks & Institutions:** 
  - Aareal Bank, Volksbank, Instabank
  - Hausbank München eG
  - DKB, Sparkassen, PSD Banken
- **Property Managers:** Various software solutions (PropTech)

## Technical Standards
- **Schema:** JSON Schema (draft-07 or later).
- **Communication:** RESTful API design.
- **Legal Compliance:** BGB § 551 (Rental deposits), Steuer-ID validation (Kapitalertragsteuer), eID integration (Online-Ausweisfunktion).

## Directory Structure
- `/`: Schema definitions and documentation.
- `/examples/`: Sample JSON payloads for different deposit types.

## Conventions
- Use `snake_case` for JSON properties.
- Ensure all monetary values include a `currency` (ISO 4217).
- Dates must follow ISO 8601 format.
- Maintain modularity in `provider` objects to allow easy extension for new banks or insurers.
