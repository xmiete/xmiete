/*
 * Copyright 2026 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
