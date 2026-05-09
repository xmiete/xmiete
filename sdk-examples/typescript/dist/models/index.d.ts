export interface Meta {
    version: string;
    timestamp: string;
    external_id?: string;
}
export interface WalletMetadata {
    issuer?: string;
    issuer_id?: string;
    wallet_provider?: string;
    credential_type?: string;
    credential_format?: string;
    assurance_level?: string;
    presentation_id?: string;
    issuing_country?: string;
    verified_at?: string;
    expires_at?: string;
}
export interface Address {
    street: string;
    zip: string;
    city: string;
    country: string;
}
export interface Tenant {
    first_name: string;
    last_name: string;
    email: string;
    tax_id?: string;
    eid_status: string;
    wallet_metadata?: WalletMetadata;
    address: Address;
}
export interface Landlord {
    name: string;
    type: string;
    iban?: string;
}
export interface Property {
    address: Address;
    unit_id?: string;
}
export interface InstallmentScheduleEntry {
    installment_number: number;
    due_date: string;
    paid_date?: string;
    amount?: number;
}
export interface InstallmentPlan {
    total_installments: number;
    installment_amount?: number;
    schedule?: InstallmentScheduleEntry[];
}
export interface DepositDetails {
    amount: number;
    currency: string;
    type: string;
    lifecycle_state: string;
    installment_plan?: InstallmentPlan;
}
export interface Pledge {
    pledge_date?: string;
    statutory_basis: string;
    is_confirmed_by_bank: boolean;
}
export interface InterestRateEntry {
    rate: number;
    effective_from: string;
    effective_to?: string;
}
export interface Trusteeship {
    account_type?: string;
    trustee_entity?: string;
    trust_account_iban?: string;
    insolvency_protection_confirmed: boolean;
    insolvency_protection_confirmed_date?: string;
    statutory_basis?: string;
    interest_rate?: number;
    interest_rate_history?: InterestRateEntry[];
    accrued_interest?: number;
    interest_calculated_at?: string;
    total_balance?: number;
}
export interface Provider {
    provider_type: string;
    executing_entity: string;
    brand_name?: string;
    insurance_policy_number?: string;
    custom_fields?: Record<string, unknown>;
}
export interface HistoryEntry {
    state: string;
    timestamp: string;
    actor: string;
    comment?: string;
}
export interface Deposit {
    meta: Meta;
    tenant: Tenant;
    landlord: Landlord;
    property: Property;
    deposit_details: DepositDetails;
    pledge?: Pledge;
    trusteeship?: Trusteeship;
    provider: Provider;
    history: HistoryEntry[];
}
