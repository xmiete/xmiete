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

export type EIDStatus = 'NOT_STARTED' | 'PENDING' | 'VERIFIED' | 'FAILED' | 'EXPIRED';

export const EID_STATUSES: readonly EIDStatus[] = [
  'NOT_STARTED',
  'PENDING',
  'VERIFIED',
  'FAILED',
  'EXPIRED',
];

/** Initiates an eID session for a tenant. */
export interface VerificationRequest {
  depositId: string;
  tenantEmail: string;
  redirectUri: string;
  clientId: string;
}

/**
 * Returned by the eID provider after session creation.
 * Redirect the tenant's browser to authorizationUrl within the validity window.
 */
export interface VerificationSession {
  sessionId: string;
  authorizationUrl: string;
  expiresAt: Date;
}

/**
 * Sent to PATCH /deposits/{id}/identity once verification completes.
 * Only providerReference is stored — never raw PII from the eID chip.
 */
export interface KYCUpdatePayload {
  eidStatus: EIDStatus;
  verificationTimestamp: Date;
  providerReference: string;
}

/** Parsed body of a POST from the eID provider to your webhook endpoint. */
export interface WebhookEvent {
  sessionId: string;
  depositId: string;
  status: EIDStatus;
  providerReference: string;
  completedAt: Date;
  errorCode: string;
}

/**
 * Interface a bank must implement to integrate an eID provider.
 * Implement this to swap in AusweisApp2, Authada, SkIDentity, Bundesdruckerei,
 * or any custom BSI TR-03130 compatible provider without changing any other SDK code.
 */
export interface IdentityVerifier {
  initiateVerification(req: VerificationRequest): Promise<VerificationSession>;
  updateDepositKYCStatus(depositId: string, payload: KYCUpdatePayload, bearerToken: string): Promise<void>;
}
