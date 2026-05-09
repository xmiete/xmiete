/**
 * EidVerificationService manages eID verification sessions against a BSI TR-03130
 * compatible eID provider (e.g., AusweisApp2 SDK, Authada, SkIDentity).
 *
 * Flow:
 *  1. Call initiateVerification → receive VerificationSession with authorizationUrl
 *  2. Redirect the tenant's browser to authorizationUrl
 *  3. eID provider POSTs to your webhook → handled by WebhookHandler
 *  4. WebhookHandler calls updateDepositKYCStatus to finalize the deposit state
 */
import type { IdentityVerifier, KYCUpdatePayload, VerificationRequest, VerificationSession } from './models';
export declare class VerificationService implements IdentityVerifier {
    private readonly eidProviderBaseUrl;
    private readonly xmieteApiBaseUrl;
    constructor(eidProviderBaseUrl: string, xmieteApiBaseUrl: string);
    /** Creates an eID session and returns the authorization redirect URL. */
    initiateVerification(req: VerificationRequest): Promise<VerificationSession>;
    /**
     * Pushes the verified eID result to the XMiete API.
     * Only providerReference is forwarded — never raw PII from the eID chip.
     */
    updateDepositKYCStatus(depositId: string, payload: KYCUpdatePayload, bearerToken: string): Promise<void>;
}
