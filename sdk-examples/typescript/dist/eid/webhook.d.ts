import type { IdentityVerifier, WebhookEvent } from './models';
export declare class EidWebhookError extends Error {
    constructor(message: string);
}
export declare const ErrMissingSignature: EidWebhookError;
export declare const ErrSignatureMismatch: EidWebhookError;
export declare const ErrMissingFields: EidWebhookError;
export declare class WebhookHandler {
    private readonly service;
    private readonly bearerToken;
    private readonly onComplete?;
    constructor(service: IdentityVerifier, bearerToken: string, onComplete?: (event: WebhookEvent) => void);
    /**
     * Validates the HMAC signature, parses the event, and dispatches the result.
     * rawBody must be the unmodified request body; signature is the X-Signature header value.
     */
    handleWebhook(rawBody: Buffer | string, signature: string, webhookSecret: string): Promise<void>;
    private dispatch;
}
export declare function verifyHmac(body: Buffer, signature: string, secret: string): void;
export declare function parseWebhookEvent(data: Buffer): WebhookEvent;
