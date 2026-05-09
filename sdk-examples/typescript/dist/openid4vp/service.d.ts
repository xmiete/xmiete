import type { VerifiedClaims, VpRequestResult } from './models';
export interface VpVerifier {
    /**
     * Creates a VP request for a DepositPledgeAttestation presentation.
     * Persist result.nonce and pass it to verifyVpToken.
     */
    buildVpRequest(depositId: string, responseUri: string): Promise<VpRequestResult>;
    /**
     * Verifies a vp_token received from the wallet.
     * vpToken is the SD-JWT VP wire format: issuerJWT~disc1~...~discN~kbJWT
     * expectedNonce must match the nonce from the corresponding buildVpRequest call.
     */
    verifyVpToken(vpToken: string, expectedNonce: string, responseUri: string): Promise<VerifiedClaims>;
}
export declare class VpVerifierService implements VpVerifier {
    private readonly clientId;
    private readonly jwksUri;
    constructor(clientId: string, jwksUri: string);
    buildVpRequest(depositId: string, responseUri: string): Promise<VpRequestResult>;
    verifyVpToken(vpToken: string, expectedNonce: string, responseUri: string): Promise<VerifiedClaims>;
}
