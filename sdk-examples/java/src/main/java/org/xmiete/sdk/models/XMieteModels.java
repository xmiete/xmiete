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

package org.xmiete.sdk.models;

import java.time.OffsetDateTime;
import java.util.List;

public record Deposit(
    Meta meta,
    Tenant tenant,
    Landlord landlord,
    Property property,
    DepositDetails depositDetails,
    Pledge pledge,
    Provider provider,
    List<HistoryEntry> history
) {}

record Meta(String version, OffsetDateTime timestamp, String externalId) {}

record Tenant(
    String firstName,
    String lastName,
    String email,
    String taxId,
    String eidStatus,
    Address address
) {}

record Address(String street, String zip, String city, String country) {}

record Landlord(String name, String type, String iban) {}

record Property(Address address, String unitId) {}

record DepositDetails(
    Double amount,
    String currency,
    String type,
    String lifecycleState
) {}

record Pledge(String pledgeDate, String legalReference, boolean isConfirmedByBank) {}

record Provider(
    String providerType,
    String executingEntity,
    String brandName,
    String insurancePolicyNumber
) {}

record HistoryEntry(String state, OffsetDateTime timestamp, String actor, String comment) {}
