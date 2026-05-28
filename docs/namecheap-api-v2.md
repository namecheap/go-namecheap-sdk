# Namecheap API v2 Reference

> **Added:** 2026-05-28
> **Source:** https://www.namecheap.com/support/api/methods/
> **Scope:** All methods available under APIv2 — domains, domains.dns, domains.ns, domains.transfer, ssl, users, users.address, domainprivacy (whoisguard).
> This file is the authoritative reference for implementing new SDK methods. Cross-check parameter names, types, and required flags here before writing Go structs or tests.

---

# Namecheap API Documentation

> **Base URLs:**
> - Sandbox: `https://api.sandbox.namecheap.com/xml.response`
> - Production: `https://api.namecheap.com/xml.response`

> All API calls return XML responses. Use HTTP GET with query parameters.

---

## Introduction

The Namecheap API allows you to build applications that integrate with your Namecheap account, enabling programmatic domain search, registration, SSL purchase, and more.

### Authentication

Every API call requires the following global parameters:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| ApiUser | String | Yes | Username required to access the API |
| ApiKey | String | Yes | Password required to access the API |
| Command | String | Yes | Command for execution |
| UserName | String | Yes | The Username on which a command is executed (usually same as ApiUser) |
| ClientIp | String | Yes | IPv4 address of the server making the API call |

**Sample API Call:**
```
https://api.namecheap.com/xml.response?ApiUser=user&ApiKey=key&UserName=user&ClientIp=1.2.3.4&Command=namecheap.domains.check&DomainList=example.com
```

---

## Table of Contents

- [domains](#domains)
- [domains.dns](#domainsdns)
- [domains.ns](#domainsns)
- [domains.transfer](#domainstransfer)
- [ssl](#ssl)
- [users](#users)
- [users.address](#usersaddress)
- [domainprivacy](#domainprivacy)

---

## domains

### namecheap.domains.getList

Returns a list of domains for the particular user.

**Command:** `namecheap.domains.getList`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| ListType | String | 10 | No | Possible values: ALL, EXPIRING, or EXPIRED. Default: ALL |
| SearchTerm | String | 70 | No | Keyword to look for in the domain list |
| Page | Number | 10 | No | Page to return. Default: 1 |
| PageSize | Number | 10 | No | Number of domains per page. Min: 10, Max: 100. Default: 20 |
| SortBy | String | 50 | No | Possible values: NAME, NAME_DESC, EXPIREDATE, EXPIREDATE_DESC, CREATEDATE, CREATEDATE_DESC |

#### Response Parameters

| Name | Description |
|------|-------------|
| ID | Unique integer value that represents the domain |
| Name | Registered domain name |
| User | User account under which the domain is registered |
| Created | Domain creation date |
| Expires | Domain expiration date |
| IsExpired | True/False - indicates whether the domain is expired |
| IsLocked | True/False - indicates whether the domain is locked |
| AutoRenew | True/False - indicates whether auto-renew is set |
| WhoisGuard | Returns the domain privacy status |
| IsPremium | Indicates whether the domain name is premium |
| IsOurDNS | True if Namecheap BasicDNS or PremiumDNS are used |

#### Error Codes

| Code | Description |
|------|-------------|
| 5050169 | Unknown exceptions |

---

### namecheap.domains.getContacts

Gets contact information of the requested domain.

**Command:** `namecheap.domains.getContacts`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| DomainName | String | 70 | Yes | Domain to get contacts |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | The registered domain name |
| DomainnameID | A unique integer value that represents the domain |
| Readonly | True/False - indicates whether contact information is read-only |

#### Error Codes

| Code | Description |
|------|-------------|
| 2019166 | Domain not found |
| 2016166 | Domain is not associated with your account |
| 4019337 | Unable to retrieve domain contacts |
| 3016166 | Domain is not associated with Enom |
| 5050900 | Unknown exceptions |

---

### namecheap.domains.create

Registers a new domain name.

**Command:** `namecheap.domains.create`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| DomainName | String | 70 | Yes | Domain name to register |
| Years | Number | 2 | Yes | Number of years to register. Default: 2 |
| PromotionCode | String | 20 | No | Promotional (coupon) code |
| RegistrantFirstName | String | 255 | Yes | First name of Registrant |
| RegistrantLastName | String | 255 | Yes | Last name of Registrant |
| RegistrantOrganizationName | String | 255 | No | Organization of Registrant |
| RegistrantJobTitle | String | 255 | No | Job title of Registrant |
| RegistrantAddress1 | String | 255 | Yes | Address1 of Registrant |
| RegistrantAddress2 | String | 255 | No | Address2 of Registrant |
| RegistrantCity | String | 50 | Yes | City of Registrant |
| RegistrantStateProvince | String | 50 | Yes | State/Province of Registrant |
| RegistrantPostalCode | String | 50 | Yes | Postal code of Registrant |
| RegistrantCountry | String | 50 | Yes | Country of Registrant (two-letter code) |
| RegistrantPhone | String | 50 | Yes | Phone number in format +NNN.NNNNNNNNNN |
| RegistrantEmailAddress | String | 255 | Yes | Email address of Registrant |
| TechFirstName, TechLastName, ... | String | varies | Yes | Technical contact details (same fields as Registrant) |
| AdminFirstName, AdminLastName, ... | String | varies | Yes | Admin contact details (same fields as Registrant) |
| AuxBillingFirstName, ... | String | varies | Yes | Billing contact details (same fields as Registrant) |
| Nameservers | String | 255 | No | Comma-separated custom nameservers |
| AddFreeWhoisguard | String | 3 | No | Add free domain privacy. Default: Yes |
| WGEnabled | String | 3 | No | Enable domain privacy. Default: No |
| IsPremiumDomain | Boolean | 10 | No | Indicate if domain is premium |
| PremiumPrice | Currency | 20 | No | Registration price for premium domain |
| EapFee | Currency | 20 | No | Purchase fee for premium domain during EAP |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name registered |
| Registered | Indicates whether the domain was registered successfully |
| ChargedAmount | Total amount charged for registration |
| DomainID | Unique integer representing the domain |
| OrderID | Unique integer representing the order |
| TransactionID | Unique integer representing the transaction |

---

### namecheap.domains.getTldList

Returns a list of TLDs.

**Command:** `namecheap.domains.getTldList`

No additional request parameters required.

#### Response Parameters

| Name | Description |
|------|-------------|
| Name | Indicates the top-level domain |
| NonRealTimeDomain | True/False - indicates whether registration is instant |
| MinRegisterYears | Minimum years the TLD can be registered for |
| MaxRegisterYears | Maximum years the TLD can be registered for |
| MinRenewYears | Minimum years the TLD can be renewed for |
| MaxRenewYears | Maximum years the TLD can be renewed for |
| MinTransferYears | Minimum years the TLD can be transferred for |
| MaxTransferYears | Maximum years the TLD can be transferred for |
| IsApiRegisterable | Whether the domain can be registered through API |
| IsApiRenewable | Whether the domain can be renewed through API |
| IsApiTransferable | Whether the domain can be transferred via API |
| IsEppRequired | Whether EPP code is required for this TLD |

---

### namecheap.domains.setContacts

Sets contact information for the requested domain.

**Command:** `namecheap.domains.setContacts`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| DomainName | String | 70 | Yes | Domain name to set contacts for |
| RegistrantFirstName | String | 255 | Yes | First name of Registrant |
| RegistrantLastName | String | 255 | Yes | Last name of Registrant |
| RegistrantAddress1 | String | 255 | Yes | Address1 of Registrant |
| RegistrantCity | String | 50 | Yes | City of Registrant |
| RegistrantStateProvince | String | 50 | Yes | State/Province of Registrant |
| RegistrantPostalCode | String | 50 | Yes | Postal code of Registrant |
| RegistrantCountry | String | 50 | Yes | Country of Registrant |
| RegistrantPhone | String | 50 | Yes | Phone number |
| RegistrantEmailAddress | String | 255 | Yes | Email address |
| TechFirstName, TechLastName, ... | String | varies | Yes | Technical contact (same fields) |
| AdminFirstName, AdminLastName, ... | String | varies | Yes | Admin contact (same fields) |
| AuxBillingFirstName, ... | String | varies | Yes | Billing contact (same fields) |

---

### namecheap.domains.check

Checks the availability of domains.

**Command:** `namecheap.domains.check`

#### Request Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| DomainList | String | Yes | Comma-separated list of domains to check (max 50) |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name checked |
| Available | Indicates whether the domain is available for registration |
| IsPremiumName | Indicates whether the domain name is premium |
| PremiumRegistrationPrice | Registration price for premium domain |
| PremiumRenewalPrice | Renewal price for premium domain |
| PremiumRestorePrice | Restore price for premium domain |
| PremiumTransferPrice | Transfer price for premium domain |
| IcannFee | Fee charged by ICANN |
| EapFee | Purchase fee for premium domain during EAP |

#### Error Codes

| Code | Description |
|------|-------------|
| 3031510 | Error response from Enom |
| 2011169 | Only 50 domains allowed in a single check command |

---

### namecheap.domains.reactivate

Reactivates an expired domain.

**Command:** `namecheap.domains.reactivate`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| DomainName | String | 70 | Yes | Domain name to reactivate |
| PromotionCode | String | 70 | No | Promotional code |
| YearsToAdd | Number | 2 | No | Number of years after expiry |
| IsPremiumDomain | Boolean | 10 | No | Whether domain is premium |
| PremiumPrice | Currency | 20 | No | Reactivation price for premium domain |

#### Response Parameters

| Name | Description |
|------|-------------|
| DomainName | Domain name being reactivated |
| IsSuccess | Whether domain was reactivated successfully |
| ChargedAmount | Total amount charged for reactivation |
| OrderID | Unique integer representing the order |
| TransactionID | Unique integer representing the transaction |

---

### namecheap.domains.renew

Renews an expiring domain.

**Command:** `namecheap.domains.renew`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| DomainName | String | 70 | Yes | Domain name to renew |
| Years | Number | 2 | Yes | Number of years to renew |
| PromotionCode | String | 20 | No | Promotional code |
| IsPremiumDomain | Boolean | 10 | No | Whether domain is premium |
| PremiumPrice | Currency | 20 | No | Renewal price for premium domain |

#### Response Parameters

| Name | Description |
|------|-------------|
| DomainName | Domain name being renewed |
| DomainID | Unique integer representing the domain |
| Renew | Whether domain was renewed successfully |
| ChargedAmount | Total amount charged for renewal |
| OrderID | Unique integer representing the order |
| TransactionID | Unique integer representing the transaction |

---

### namecheap.domains.getRegistrarLock

Gets the RegistrarLock status for the requested domain.

**Command:** `namecheap.domains.getRegistrarLock`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| DomainName | String | 70 | Yes | Domain name to get status for |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name checked |
| RegistrarLockStatus | True/False - True means registrar lock is set |

---

### namecheap.domains.setRegistrarLock

Sets the RegistrarLock status for a domain.

**Command:** `namecheap.domains.setRegistrarLock`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| DomainName | String | 70 | Yes | Domain name to set status for |
| LockAction | String | 70 | No | Possible values: LOCK, UNLOCK. Default: LOCK |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| IsSuccess | Whether registrar lock was set successfully |

---

### namecheap.domains.getInfo

Returns information about the requested domain.

**Command:** `namecheap.domains.getInfo`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| DomainName | String | 70 | Yes | Domain name to get information for |
| HostName | String | 255 | No | Hosted domain name for which domain info is requested |

#### Response Parameters

| Name | Description |
|------|-------------|
| Status | Domain status: OK, Locked, Expired |
| ID | Unique integer representing the domain |
| DomainName | Domain name |
| OwnerName | User account under which domain is registered |
| IsOwner | Whether the API user is the domain owner |
| IsPremium | Whether the domain name is premium |

#### Error Codes

| Code | Description |
|------|-------------|
| 5019169 | Unknown exceptions |
| 2030166 | Domain is invalid |
| 4011103 | DomainName not available / UserName not available / Access denied |

---

## domains.dns

### namecheap.domains.dns.setDefault

Sets domain to use Namecheap's default DNS servers. Required for free services like Host record management, URL forwarding, email forwarding, dynamic DNS, and other value-added services.

**Command:** `namecheap.domains.dns.setDefault`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| SLD | String | 70 | Yes | SLD of the DomainName |
| TLD | String | 10 | Yes | TLD of the DomainName |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| Updated | Whether default nameservers were set successfully |

#### Error Codes

| Code | Description |
|------|-------------|
| 2019166 | Domain not found |
| 2016166 | Domain is not associated with your account |
| 2030166 | Edit permission for domain is not supported |

---

### namecheap.domains.dns.setCustom

Sets domain to use custom DNS servers. NOTE: Services like URL forwarding, Email forwarding, Dynamic DNS will not work with custom nameservers.

**Command:** `namecheap.domains.dns.setCustom`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| SLD | String | 70 | Yes | SLD of the DomainName |
| TLD | String | 10 | Yes | TLD of the DomainName |
| Nameservers | String | 1200 | Yes | Comma-separated list of nameservers |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| Updated | Whether custom nameservers were set successfully |

---

### namecheap.domains.dns.getList

Gets a list of DNS servers associated with the requested domain.

**Command:** `namecheap.domains.dns.getList`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| SLD | String | 70 | Yes | SLD of the DomainName |
| TLD | String | 10 | Yes | TLD of the DomainName |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| IsUsingOurDNS | Whether Namecheap's default nameservers are used |

---

### namecheap.domains.dns.getHosts

Retrieves DNS host record settings for the requested domain.

**Command:** `namecheap.domains.dns.getHosts`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| SLD | String | 70 | Yes | SLD of the domain |
| TLD | String | 10 | Yes | TLD of the domain |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| IsUsingOurDNS | Whether Namecheap's default nameservers are used |
| HostID | Unique ID of the host record |
| Name | Domain/subdomain for which host record is set |
| Type | Type of host record |
| Address | Value set for the host record (IP or URL) |
| MXPref | MX preference number |
| TTL | TTL value for the host record |

---

### namecheap.domains.dns.getEmailForwarding

Gets email forwarding settings for the requested domain.

**Command:** `namecheap.domains.dns.getEmailForwarding`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| DomainName | String | 70 | Yes | Domain name to get settings |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| Mailbox | The email forwarding mailbox created |

---

### namecheap.domains.dns.setEmailForwarding

Sets email forwarding for a domain name.

**Command:** `namecheap.domains.dns.setEmailForwarding`

#### Request Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| DomainName | String | Yes | Domain name to set settings for |
| MailBox[1..n] | String | Yes | Mailbox for email forwarding (e.g. example@domain.com) |
| ForwardTo[1..n] | String | Yes | Email address to forward to (e.g. example@gmail.com) |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| IsSuccess | Whether email forwarding was set successfully |

---

### namecheap.domains.dns.setHosts

Sets DNS host records settings for the requested domain.

**Command:** `namecheap.domains.dns.setHosts`

#### Request Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| SLD | String | Yes | SLD of the domain |
| TLD | String | Yes | TLD of the domain |
| HostName[1..n] | String | Yes | Sub-domain/hostname for the record |
| RecordType[1..n] | String | Yes | Possible values: A, AAAA, ALIAS, CAA, CNAME, MX, MXE, NS, TXT, URL, URL301, FRAME |
| Address[1..n] | String | Yes | URL or IP address based on RecordType |
| MXPref[1..n] | String | Yes | MX preference (for MX records only) |
| EmailType | String | Yes | Values: MXE, MX, FWD, OX |
| TTL[1..n] | String | No | Time to live for record (300-60000) |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| IsSuccess | Whether DNS host records were set successfully |

---

## domains.ns

### namecheap.domains.ns.create

Creates a new nameserver.

**Command:** `namecheap.domains.ns.create`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| SLD | String | 70 | Yes | SLD of the DomainName |
| TLD | String | 10 | Yes | TLD of the DomainName |
| Nameserver | String | 150 | Yes | Nameserver to create |
| IP | String | 15 | Yes | Nameserver IP address |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| Nameserver | Nameserver that was created |
| IP | IP address set for the nameserver |
| IsSuccess | Whether nameserver was created successfully |

---

### namecheap.domains.ns.delete

Deletes a nameserver associated with the requested domain.

**Command:** `namecheap.domains.ns.delete`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| SLD | String | 70 | Yes | SLD of the DomainName |
| TLD | String | 10 | Yes | TLD of the DomainName |
| Nameserver | String | 150 | Yes | Nameserver to delete |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| Nameserver | Nameserver that was deleted |
| IsSuccess | Whether nameserver was deleted successfully |

---

### namecheap.domains.ns.getInfo

Retrieves information about a registered nameserver.

**Command:** `namecheap.domains.ns.getInfo`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| SLD | String | 70 | Yes | SLD of the DomainName |
| TLD | String | 10 | Yes | TLD of the DomainName |
| Nameserver | String | 150 | Yes | Nameserver to get info for |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| Nameserver | Returns the nameserver |
| IP | IP address set for the nameserver |

---

### namecheap.domains.ns.update

Updates the IP address of a registered nameserver.

**Command:** `namecheap.domains.ns.update`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| SLD | String | 70 | Yes | SLD of the Domain Name |
| TLD | String | 10 | Yes | TLD of the Domain Name |
| Nameserver | String | 150 | Yes | Nameserver to update |
| OldIP | String | 15 | Yes | Existing IP address |
| IP | String | 15 | Yes | New IP address |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domain | Domain name |
| Nameserver | Nameserver that was updated |
| IsSuccess | Whether nameserver was updated successfully |

---

## domains.transfer

### namecheap.domains.transfer.create

Transfers a domain to Namecheap. Supported TLDs: .biz, .ca, .cc, .co, .com, .com.es, .com.pe, .es, .in, .info, .me, .mobi, .net, .net.pe, .nom.es, .org, .org.es, .org.pe, .pe, .tv, .us.

**Command:** `namecheap.domains.transfer.create`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| DomainName | String | 70 | Yes | Domain name to transfer |
| Years | String | 1 | Yes | Should be set to 1 year only |
| EPPCode | String | 20 | Yes | EPP code required for most TLDs |
| PromotionCode | String | 20 | No | Promotional code |
| AddFreeWhoisguard | String | 3 | No | Add free domain privacy. Default: Yes |
| WGenable | String | 3 | No | Enable domain privacy. Default: No |

#### Response Parameters

| Name | Description |
|------|-------------|
| DomainName | Domain name being transferred |
| TransferID | Unique integer representing the transfer |
| StatusID | Status code of the transfer |
| OrderID | Unique integer representing the order |
| TransactionID | Unique integer representing the transaction |
| ChargedAmount | Amount charged for the transfer |

---

### namecheap.domains.transfer.getStatus

Gets the status of a particular transfer.

**Command:** `namecheap.domains.transfer.getStatus`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| TransferID | Number | 10 | Yes | The unique Transfer ID from transfer creation |

#### Response Parameters

| Name | Description |
|------|-------------|
| TransferID | Unique integer representing the transfer |
| StatusID | Integer indicating current transfer status |
| Status | Transfer status description |

---

### namecheap.domains.transfer.updateStatus

Updates the status of a particular transfer. Allows re-submission after releasing registry lock.

**Command:** `namecheap.domains.transfer.updateStatus`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| TransferID | Number | 10 | Yes | The unique Transfer ID |
| Resubmit | String | 5 | Yes | The value "true" resubmits the transfer |

#### Response Parameters

| Name | Description |
|------|-------------|
| TransferID | Unique integer representing the transfer |
| Resubmit | Whether the transfer order was resubmitted |

---

### namecheap.domains.transfer.getList

Gets the list of domain transfers.

**Command:** `namecheap.domains.transfer.getList`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| ListType | String | 10 | No | Possible values: ALL, INPROGRESS, CANCELLED, COMPLETED. Default: ALL |
| SearchTerm | String | 70 | No | Keyword (domain name) to search |
| Page | Number | 10 | No | Page to return. Default: 1 |
| PageSize | Number | 10 | No | Transfers per page. Min: 10, Max: 100. Default: 10 |
| SortBy | String | 50 | No | Possible values: DOMAINNAME, DOMAINNAME_DESC, TRANSFERDATE, TRANSFERDATE_DESC, STATUSDATE, STATUSDATE_DESC |

#### Response Parameters

| Name | Description |
|------|-------------|
| TransferID | Unique integer representing the transfer |
| Domainname | Domain name associated with the transfer |
| User | User account to which the domain is transferred |
| TransferDate | Date the transfer was initiated |
| OrderID | Unique integer representing the order |
| StatusID | Integer indicating current transfer status |
| Status | Transfer status description |

---

## ssl

### namecheap.ssl.create

Creates a new SSL certificate.

**Command:** `namecheap.ssl.create`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| Years | Number | 1 | Yes | Number of years for SSL. Allowed values: 1,2,3,4,5 |
| Type | String | 50 | Yes | SSL product name |
| SANStoADD | Number | 2 | No | Number of add-on domains for multi-domain certificates |
| PromotionCode | String | 20 | No | Promotional code |

#### Response Parameters

| Name | Description |
|------|-------------|
| IsSuccess | Whether SSL order was successful |
| OrderID | Unique integer representing the order |
| TransactionID | Unique integer representing the transaction |
| ChargedAmount | Amount charged for the order |
| CertificateID | Unique integer representing the SSL certificate |
| Created | Date the certificate was created |
| SSLType | Type of SSL certificate |

---

### namecheap.ssl.getList

Returns a list of SSL certificates for the user.

**Command:** `namecheap.ssl.getList`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| ListType | String | 50 | No | Values: ALL, Processing, EmailSent, TechnicalProblem, InProgress, Completed, Deactivated, Active, Cancelled, NewPurchase, NewRenewal. Default: All |
| SearchTerm | String | 70 | No | Keyword to look for |
| Page | Number | 10 | No | Page to return. Default: 1 |
| PageSize | Number | 100 | No | Certificates per page. Min: 10, Max: 100. Default: 20 |
| SortBy | String | 20 | No | Values: PURCHASEDATE, PURCHASEDATE_DESC, SSLTYPE, SSLTYPE_DESC, EXPIREDATETIME, EXPIREDATETIME_DESC, Host_Name, Host_Name_DESC |

#### Response Parameters

| Name | Description |
|------|-------------|
| CertificateID | Unique integer representing the SSL certificate |
| HostName | Common name for which SSL is used |
| SSLType | Type of SSL |
| PurchaseDate | Date the certificate was purchased |
| ExpireDate | Date the certificate expires |

---

### namecheap.ssl.parseCSR

Parses the CSR (Certificate Signing Request).

**Command:** `namecheap.ssl.parseCSR`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| csr | String | 2000 | Yes | Certificate Signing Request |
| CertificateType | String | 50 | No* | Type of SSL Certificate |

#### Response Parameters

| Name | Description |
|------|-------------|
| Common name | Hostname for which SSL is applied |
| Domain name | Domain name for which SSL is applied |
| Country | Country of the SSL applicant |
| Organisation Unit | Organisation unit of the SSL applicant |
| Organisation | Organisation of the SSL applicant |
| State | State information |
| Locality | Locality information |
| Email | Email address of the SSL applicant |

---

### namecheap.ssl.getApproverEmailList

Gets approver email list for the requested certificate.

**Command:** `namecheap.ssl.getApproverEmailList`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| DomainName | String | 70 | Yes | Domain name to get the list for |
| CertificateType | String | 50 | Yes | Type of SSL certificate |

#### Response Parameters

| Name | Description |
|------|-------------|
| DomainEmails | Domain Whois email address |
| GenericEmails | Generic email addresses for the domain |
| ManualEmails | Additional emails from provider |

---

### namecheap.ssl.activate

Activates a newly purchased SSL certificate.

**Command:** `namecheap.ssl.activate`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| CertificateID | Number | 10 | Yes | Unique identifier of SSL certificate to activate |
| CSR | String | 32767 | Yes | Certificate Signing Request (CSR) |
| AdminEmailAddress | String | 255 | Yes | Email to send signed SSL certificate file to |
| WebServerType | String | 50 | No | Server software type (apacheopenssl, nginx, iis, etc.) |
| UniqueValue | String | 20 | No | Unique identifier for SSL issue/reissue request |

---

### namecheap.ssl.resendApproverEmail

Resends the approver email. Also serves as a retry mechanism for HTTP/DNS validation.

**Command:** `namecheap.ssl.resendApproverEmail`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| CertificateID | String | 10 | Yes | Unique certificate ID from ssl.create |

#### Response Parameters

| Name | Description |
|------|-------------|
| ID | Unique integer representing the SSL certificate |
| IsSuccess | Whether approver email was resent successfully |

---

### namecheap.ssl.getInfo

Retrieves information about the requested SSL certificate.

**Command:** `namecheap.ssl.getInfo`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| CertificateID | Number | 10 | Yes | Unique ID of the SSL certificate |
| Returncertificate | String | 10 | No | Flag for returning certificate in response |
| Returntype | String | 10 | No | Type of returned certificate: "Individual" (X.509) or "PKCS7" |

#### Certificate Status Values

| Status | Description |
|--------|-------------|
| Active | Certificate is activated |
| Newpurchase | Initial status after purchase; use ssl.activate next |
| Newrenewal | Initial status after renewal purchase |
| Purchased | Certificate activated and awaiting issuance |
| Purchaseerror | Error while processing the certificate |
| Cancelled | Certificate is cancelled |

---

### namecheap.ssl.renew

Renews an SSL certificate.

**Command:** `namecheap.ssl.renew`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| CertificateID | Number | 9 | Yes | Unique ID of the SSL certificate to renew |
| Years | Number | 1 | Yes | Number of years for renewal. Allowed values: 1,2,3,4,5 |
| SSLType | String | 50 | Yes | SSL product name |
| PromotionCode | String | 20 | No | Promotional code |

#### Response Parameters

| Name | Description |
|------|-------------|
| CertificateID | Unique integer for the renewal certificate |
| Years | Number of years valid once issued |
| OrderID | Unique integer for the renewal order |
| TransactionID | Unique integer for the renewal transaction |
| ChargedAmount | Amount charged for the order |

---

### namecheap.ssl.reissue

Reissues an SSL certificate.

**Command:** `namecheap.ssl.reissue`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| CertificateID | Number | 10 | Yes | Unique identifier of SSL certificate to reissue |
| CSR | String | 32767 | Yes | Certificate Signing Request (CSR) |
| AdminEmailAddress | String | 255 | No | Email address (cannot be changed from initial activation) |
| WebServerType | String | 50 | No | Server software type (apacheopenssl, nginx, iis, etc.) |
| UniqueValue | String | 20 | No | Unique identifier for the reissue request |

---

### namecheap.ssl.resendfulfillmentemail

Resends the fulfilment email containing the certificate.

**Command:** `namecheap.ssl.resendfulfillmentemail`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| CertificateID | String | 10 | Yes | Unique certificate ID from ssl.create |

#### Response Parameters

| Name | Description |
|------|-------------|
| ID | Unique integer representing the SSL certificate |
| IsSuccess | Whether the fulfillment email was resent successfully |

---

### namecheap.ssl.purchasemoresans

Purchases more add-on domains for an already purchased certificate.

**Command:** `namecheap.ssl.purchasemoresans`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| CertificateID | Number | 10 | Yes | ID of the certificate |
| NumberOfSANSToAdd | Number | 2 | Yes | Number of add-on domains to order |

#### Response Parameters

| Name | Description |
|------|-------------|
| IsSuccess | Whether more SANs were purchased |
| OrderID | Unique integer representing the order |
| TransactionID | Unique integer representing the transaction |
| ChargedAmount | Amount charged |
| CertificateID | Unique integer representing the SSL |
| SSLType | Type of SSL certificate |
| SANSCount | Number of add-on domains |

---

### namecheap.ssl.revokecertificate

Revokes a re-issued SSL certificate.

**Command:** `namecheap.ssl.revokecertificate`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| CertificateID | Number | 10 | Yes | ID of the certificate to revoke |
| CertificateType | String | 50 | Yes | Type of SSL Certificate |

#### Response Parameters

| Name | Description |
|------|-------------|
| IsSuccess | Whether certificate was revoked successfully |
| CertificateID | Unique integer representing the certificate |

---

### namecheap.ssl.editDCVMethod

Sets new domain control validation (DCV) method for a certificate or serves as a retry mechanism.

**Command:** `namecheap.ssl.editDCVMethod`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| CertificateID | Number | 10 | Yes | Unique ID of the SSL certificate |
| DCVMethod* | String | 255 | Yes | DCV method to validate domain control |
| DNSNames** | String | 3500 | Yes | Comma-separated list of domain names |
| DCVMethods** | String | 3500 | Yes | Comma-separated list of DCV methods |

#### Response Parameters

| Name | Description |
|------|-------------|
| ID | Unique integer representing the SSL certificate |
| IsSuccess | Whether the certificate DCV was updated |
| HttpDCValidation.ValueAvailable | True/False - indicates if HTTP_CSR_HASH was set for at least one domain |
| FileName | File name for HTTP DCV text file |
| FileContent | File content for HTTP DCV text file |

---

## users

### namecheap.users.getPricing

Returns pricing information for a requested product type.

**Command:** `namecheap.users.getPricing`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| ProductType | String | 20 | Yes | Product type to get pricing (e.g., DOMAIN, SSLCERTIFICATE) |
| ProductCategory | String | 20 | No | Specific category within product type |
| PromotionCode | String | 20 | No | Promotional code |
| ActionName | String | 20 | No | Specific action (e.g., REGISTER, RENEW, TRANSFER) |
| ProductName | String | 20 | No | Name of the product |

#### Response Parameters

| Name | Description |
|------|-------------|
| ProductType Name | Type of product |
| ProductCategory Name | Category type |
| Product Name | Name of the product |
| Duration | Duration of the product |
| DurationType | Duration type |
| Price | Final price (from regular, userprice, special, promo, or tier price) |
| RegularPrice | Regular price |
| YourPrice | User's price |

---

### namecheap.users.getBalances

Gets information about funds in the user's account.

**Command:** `namecheap.users.getBalances`

No additional request parameters required.

#### Response Parameters

| Name | Description |
|------|-------------|
| Currency | Currency in which the price is listed |
| AvailableBalance | Total amount available in account |
| AccountBalance | Total amount in account (same as AvailableBalance) |
| EarnedAmount | Amount earned from Marketplace sales |
| WithdrawableAmount | Amount available for withdrawal |
| FundsRequiredForAutoRenew | Amount required for auto-renewal |

---

### namecheap.users.changePassword

Changes password of a user account. Works in two ways: with OldPassword/NewPassword, or with ResetCode (from resetPassword API).

**Command:** `namecheap.users.changePassword`

#### Method 1: Using Old Password

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| OldPassword | String | 20 | Yes | Old password of the user |
| NewPassword | String | 20 | Yes | New password of the user |

#### Method 2: Using Reset Code

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| ResetCode | String | 50 | Yes | Reset code from namecheap.users.resetpassword |
| NewPassword | String | 20 | Yes | New password of the user |

#### Response Parameters

| Name | Description |
|------|-------------|
| Success | Whether password was changed successfully |
| UserID | Unique integer representing the user |

---

### namecheap.users.update

Updates user account information.

**Command:** `namecheap.users.update`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| FirstName | String | 70 | Yes | First name |
| LastName | String | 70 | Yes | Last name |
| JobTitle | String | 60 | No | Job designation |
| Organization | String | 60 | No | Organization |
| Address1 | String | 60 | Yes | Street address 1 |
| Address2 | String | 60 | No | Street address 2 |
| City | String | 60 | Yes | City |
| StateProvince | String | 60 | Yes | State/Province |
| Zip | String | 15 | Yes | Zip/Postal code |
| Country | String | 2 | Yes | Two-letter country code |
| EmailAddress | String | 255 | Yes | Email address |
| Phone | String | 20 | Yes | Phone number in format +NNN.NNNNNNNNNN |
| PhoneExt | String | 10 | No | Phone extension |
| Fax | String | 20 | No | Fax number in format +NNN.NNNNNNNNNN |

---

### namecheap.users.createaddfundsrequest

Creates a request to add funds through a credit card.

**Command:** `namecheap.users.createaddfundsrequest`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| Username | String | 20 | Yes | Username to add funds to |
| PaymentType | String | 30 | Yes | Allowed value: Creditcard |
| Amount | Number | 10 | Yes | Amount to add |
| ReturnUrl | String | 300 | Yes | URL to redirect user after payment |

#### Response Parameters

| Name | Description |
|------|-------------|
| TokenID | Unique ID to redirect user to add funds page |
| RedirectURL | URL for submitting credit card details |
| ReturnURL | URL to redirect after payment |

---

### namecheap.users.getAddFundsStatus

Gets the status of an add funds request.

**Command:** `namecheap.users.getAddFundsStatus`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| TokenId | String | 100 | Yes | Unique ID from createaddfundsrequest |

#### Response Parameters

| Name | Description |
|------|-------------|
| TransactionID | Unique integer representing the transaction |
| Amount | Amount added |
| Status | Status of added fund: CREATED, SUBMITTED, COMPLETED, FAILED, EXPIRED |

---

### namecheap.users.create

Creates a new account at Namecheap under this ApiUser.

**Command:** `namecheap.users.create`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| NewUserName | String | 20 | Yes | Username for the new account |
| NewUserPassword | String | 20 | Yes | Password for the new account |
| EmailAddress | String | 128 | Yes | Email address |
| IgnoreDuplicateEmailAddress | String | 10 | No | Whether to skip duplicate email check. Default: Yes |
| FirstName | String | 60 | Yes | First name |
| LastName | String | 60 | Yes | Last name |
| AcceptTerms | Number | 1 | Yes | Must be 1 to accept terms |
| AcceptNews | Number | 1 | No | 1 to receive newsletters, 0 to opt out |
| JobTitle | String | 60 | No | Job designation |
| Organization | String | 60 | No | Organization |
| Address1 | String | 60 | Yes | Street address 1 |
| Address2 | String | 60 | No | Street address 2 |
| City | String | 60 | Yes | City |
| StateProvince | String | 60 | Yes | State/Province |
| Zip | String | 15 | Yes | Zip/Postal code |
| Country | String | 2 | Yes | Two-letter country code |
| Phone | String | 20 | Yes | Phone in format +NNN.NNNNNNNNNN |
| PhoneExt | String | 10 | No | Phone extension |
| Fax | String | 20 | No | Fax number |

---

### namecheap.users.login

Validates username and password of accounts created via namecheap.users.create. Cannot be used to validate accounts created directly at namecheap.com.

**Command:** `namecheap.users.login`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| Password | String | 20 | Yes | Password of the user account |

#### Response Parameters

| Name | Description |
|------|-------------|
| Username | Username of the user account |
| LoginSuccess | Whether login was successful |

---

### namecheap.users.resetPassword

Sends a password reset link to the end user's profile email. The user must click the link to reset their password.

**Command:** `namecheap.users.resetPassword`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| FindBy | String | 20 | Yes | Possible values: EMAILADDRESS, DOMAINNAME, USERNAME |
| FindByValue | String | 20 | Yes | The username/email/domain to search by |
| EmailFromName | String | 20 | No | Sender name in reset email. Default: namecheap.com |
| EmailFrom | String | 20 | No | Sender email. Default: support@namecheap.com |
| URLPattern | String | 20 | No | URL pattern with [RESETCODE] placeholder. Default: http://namecheap.com [RESETCODE] |

#### Response Parameters

| Name | Description |
|------|-------------|
| Success | Whether password reset was initiated successfully |

---

## users.address

### namecheap.users.address.create

Creates a new address for the user.

**Command:** `namecheap.users.address.create`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| AddressName | String | 20 | Yes | Address name/label |
| DefaultYN | Number | 128 | No | 1 to set as default address, 0 otherwise |
| EmailAddress | String | 128 | Yes | Email address |
| FirstName | String | 60 | Yes | First name |
| LastName | String | 60 | Yes | Last name |
| JobTitle | String | 60 | No | Job designation |
| Organization | String | 60 | No | Organization |
| Address1 | String | 60 | Yes | Street address 1 |
| Address2 | String | 60 | No | Street address 2 |
| City | String | 60 | Yes | City |
| StateProvince | String | 60 | Yes | State/Province |
| StateProvinceChoice | String | 60 | Yes | State/Province choice |
| Zip | String | 15 | Yes | Zip/Postal code |
| Country | String | 2 | Yes | Two-letter country code |
| Phone | String | 20 | Yes | Phone in format +NNN.NNNNNNNNNN |
| PhoneExt | String | 10 | No | Phone extension |
| Fax | String | 20 | No | Fax number |

---

### namecheap.users.address.delete

Deletes an address for the user.

**Command:** `namecheap.users.address.delete`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| AddressId | Number | 20 | Yes | Unique AddressID to delete |

#### Response Parameters

| Name | Description |
|------|-------------|
| Success | Whether address was deleted successfully |
| ProfileID | Unique integer representing the address profile |
| Username | Username in question |

---

### namecheap.users.address.getInfo

Gets information for the requested addressID.

**Command:** `namecheap.users.address.getInfo`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| AddressId | Number | 20 | Yes | Unique AddressID |

#### Error Codes

| Code | Description |
|------|-------------|
| 4011331 | StatusCode for getInfo is invalid |
| 4022336 | Failed to retrieve user's address |

---

### namecheap.users.address.getList

Gets a list of addressIDs and address names associated with the user account.

**Command:** `namecheap.users.address.getList`

No additional request parameters required.

#### Response Parameters

| Name | Description |
|------|-------------|
| AddressID | Unique integer representing the address profile |
| AddressName | Name of the address profile |

---

### namecheap.users.address.setDefault

Sets the default address for the user.

**Command:** `namecheap.users.address.setDefault`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| AddressId | Number | 20 | Yes | Unique addressID to set as default |

#### Response Parameters

| Name | Description |
|------|-------------|
| Success | Whether default address was set successfully |
| AddressID | Unique integer representing the address profile |

---

### namecheap.users.address.update

Updates an address for the user.

**Command:** `namecheap.users.address.update`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| AddressId | Number | 20 | Yes | Unique address ID to update |
| AddressName | String | 20 | Yes | AddressName to update |
| DefaultYN | Number | 128 | No | 1 to set as default, 0 otherwise |
| EmailAddress | String | 128 | Yes | Email address |
| FirstName | String | 60 | Yes | First name |
| LastName | String | 60 | Yes | Last name |
| JobTitle | String | 60 | No | Job designation |
| Organization | String | 60 | No | Organization |
| Address1 | String | 60 | Yes | Street address 1 |
| Address2 | String | 60 | No | Street address 2 |
| City | String | 60 | Yes | City |
| StateProvince | String | 60 | Yes | State/Province |
| StateProvinceChoice | String | 60 | Yes | State/Province choice |
| Zip | String | 15 | Yes | Zip/Postal code |
| Country | String | 2 | Yes | Two-letter country code |
| Phone | String | 20 | Yes | Phone in format +NNN.NNNNNNNNNN |
| PhoneExt | String | 10 | No | Phone extension |
| Fax | String | 20 | No | Fax number |

---

## domainprivacy

> **Note:** The privacy service provider was renamed from WhoisGuard to WithheldforPrivacy. The API commands still use "whoisguard" naming for backward compatibility.

### namecheap.whoisguard.changeemailaddress

Changes the domain privacy email address.

**Command:** `namecheap.whoisguard.changeemailaddress`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| WhoisguardID | Number | 10 | Yes | Unique domain privacy ID to change |

#### Response Parameters

| Name | Description |
|------|-------------|
| ID | Unique integer representing domain privacy subscription |
| IsSuccess | Whether email address was changed successfully |
| WGEmail | New domain privacy email address |
| WGOldEmail | Old domain privacy email address |

---

### namecheap.whoisguard.enable

Enables domain privacy protection for the specified WhoisguardID.

**Command:** `namecheap.whoisguard.enable`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| WhoisguardID | Number | 10 | Yes | Unique domain privacy ID to enable |
| ForwardedToEmail | String | 70 | Yes | Email address to forward domain privacy emails to |

#### Response Parameters

| Name | Description |
|------|-------------|
| DomainName | Domain name for which privacy was enabled |
| IsSuccess | Whether domain privacy was enabled successfully |

---

### namecheap.whoisguard.disable

Disables domain privacy protection for the specified WhoisguardID.

**Command:** `namecheap.whoisguard.disable`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| WhoisguardID | Number | 10 | Yes | Unique domain privacy ID to disable |

#### Response Parameters

| Name | Description |
|------|-------------|
| Domainname | Domain name associated with the subscription |
| IsSuccess | Whether domain privacy was disabled successfully |

---

### namecheap.whoisguard.getlist

Gets the list of domain privacy protection subscriptions.

**Command:** `namecheap.whoisguard.getlist`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| ListType | String | 10 | No | Possible values: ALL, ALLOTED, FREE, DISCARD. Default: ALL |
| Page | Number | 10 | No | Page to return. Default: 1 |
| PageSize | Number | 20 | No | Subscriptions per page. Min: 2, Max: 100 |

#### Response Parameters

| Name | Description |
|------|-------------|
| Whoisguard ID | Unique integer representing the domain privacy subscription |
| Domainname | Domain name associated with the subscription |
| Created | Domain privacy creation date |
| Expires | Domain privacy expiry date |
| Status | Current status of the domain privacy subscription |

---

### namecheap.whoisguard.renew

Renews domain privacy protection.

**Command:** `namecheap.whoisguard.renew`

#### Request Parameters

| Name | Type | MaxLength | Required | Description |
|------|------|-----------|----------|-------------|
| WhoisguardID | String | 10 | Yes | Domain privacy ID to renew |
| Years | Number | 9 | Yes | Number of years to renew. Default: 1 |
| PromotionCode | Number | 20 | No | Promotional code |

#### Response Parameters

| Name | Description |
|------|-------------|
| WhoisGuard ID | Unique integer representing the domain privacy subscription |
| Years | Number of years to renew |
| Renew | Renewal status |
| OrderId | Unique integer representing the order |
| TransactionId | Unique integer representing the transaction |
| ChargedAmount | Amount charged for renewal |

---

## Error Codes

Common error codes across the API:

| Code | Description |
|------|-------------|
| 2019166 | Domain not found |
| 2016166 | Domain is not associated with your account |
| 2030166 | Edit permission for domain is not supported |
| 3031510 | Error response from Enom |
| 3050900 | Unknown error from Enom |
| 5050900 | Unhandled exceptions |
| 2033409 | Order chargeable for Username not found |
| 2011170 | Validation error from PromotionCode |

---

*Documentation generated from https://www.namecheap.com/support/api/methods/ — May 2026*
