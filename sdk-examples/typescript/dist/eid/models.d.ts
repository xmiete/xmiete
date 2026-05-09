export type EIDStatus = 'NOT_STARTED' | 'PENDING' | 'VERIFIED' | 'FAILED' | 'EXPIRED';
export declare const EID_STATUSES: readonly EIDStatus[];
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
