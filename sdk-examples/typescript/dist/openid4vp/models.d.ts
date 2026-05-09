/** OpenID4VP Authorization Request Object sent to the wallet. */
export interface VpRequest {
    client_id: string;
    response_type: string;
    response_mode: string;
    response_uri: string;
    nonce: string;
    state: string;
    presentation_definition: PresentationDefinition;
}
/** DIF Presentation Exchange top-level definition. */
export interface PresentationDefinition {
    id: string;
    input_descriptors: InputDescriptor[];
}
/** Describes what credential the verifier is requesting. */
export interface InputDescriptor {
    id: string;
    format: Record<string, FormatAlgs>;
    constraints: Constraints;
}
/** Lists the supported signing algorithms for a given credential format. */
export interface FormatAlgs {
    alg: string[];
}
/** Field-level constraints on the requested credential. */
export interface Constraints {
    fields: Field[];
    limit_disclosure: string;
}
/** A single JSONPath-addressed field constraint. */
export interface Field {
    path: string[];
    filter?: FieldFilter;
    optional?: boolean;
}
/** JSON Schema-style filter applied to a field value. */
export interface FieldFilter {
    type: string;
    const?: string;
}
/** Bundles the nonce (for later verification) and the request object. */
export interface VpRequestResult {
    nonce: string;
    vpRequest: VpRequest;
}
/** Claims extracted from a successfully verified DepositPledgeAttestation VP. */
export interface VerifiedClaims {
    credentialId: string;
    depositId: string;
    pledgeDate: string;
    statutoryBasis: string;
    issuingBank: string;
    depositAmount?: number;
    currency?: string;
    propertyAddress?: string;
    tenantFirstName?: string;
    tenantLastName?: string;
    pledgedUntil?: string;
    verifiedAt: Date;
}
/** Parsed payload of the SD-JWT issuer JWT. */
export interface IssuerClaims {
    credentialId: string;
    depositId: string;
    pledgeDate: string;
    statutoryBasis: string;
    issuingBank: string;
    sdHashes: string[];
    exp: number;
}
