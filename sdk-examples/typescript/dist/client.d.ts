import type { Deposit } from './models/index';
export interface XMieteClient {
    createDeposit(deposit: Deposit): Promise<Deposit>;
    getDeposit(id: string): Promise<Deposit>;
    confirmPledge(id: string, pledgeDate: string): Promise<void>;
    releaseDeposit(id: string, signatureToken: string): Promise<void>;
}
