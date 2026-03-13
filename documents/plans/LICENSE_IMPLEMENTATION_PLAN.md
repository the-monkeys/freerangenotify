# License Management Implementation Plan

## 1. Architectural Strategy: Where Should Licenses Live?

When designing a licensing model for a multi-tenant system like FreeRangeNotify, there are three potential strategies: App-level, Admin-level, and Org/Tenant-level.

### Comparison
| Level | Description | Pros | Cons | Recommendation |
| :--- | :--- | :--- | :--- | :--- |
| **App-Level** | Each application requires its own license/subscription. | Highly granular billing. | Extreme user friction. Users with multiple apps must manage multiple bills. | **Discard** for SaaS. Too complex. |
| **Admin-Level** | Global license applied to the entire FreeRangeNotify instance. | Simple. Perfect for Self-Hosted/On-Premise deployments. | Useless for multi-tenant SaaS. | **Use for Self-Hosted only**. |
| **Org/Tenant-Level** | The organization holds the license, dictating limits/features for all apps within it. | Industry standard (B2B SaaS). Reduces friction, scales well with enterprise features (SSO, RBAC). | Requires robust tenant abstraction. | **Primary Strategy for SaaS**. |

**Recommendation:** 
Implementing an **Org/Tenant-Level licensing model** is the industry standard for SaaS. Applications created under a user's *Personal Workspace* should be treated technically as a "Free-Tier Single-User Organization". If a user wants premium features (higher throughput, SLA, or premium channels), they upgrade their Org (or their Personal Workspace is converted into a licensed entity).

---

## 2. Mocking the Payment Gateway (Current Phase)

Since there is no payment gateway yet, we will introduce the concept of "Checkout" and "Billing State" without actual financial transactions. 

### Backend API Additions
1. **`GET /v1/organizations/:org_id/billing`**
   - Returns the current billing tier (e.g., `free`, `pro`, `enterprise`), quotas, and expiration dates.
2. **`POST /v1/organizations/:org_id/billing/checkout`**
   - **Mock Behavior:** Instead of returning a payment gateway session URL (e.g., Razorpay or Stripe), this endpoint will directly assign a predefined "Pro" license to the organization and return a `200 OK` with a success redirect URL indicating payment was "successful".

### Database Schema Updates (Tenant/Organization)
Extend the existing Tenant/Organization models to include billing fields:
```go
type Tenant struct {
    // Existing fields...
    ID string 
    
    // New Licensing Fields
    BillingTier   string    `db:"billing_tier"`    // e.g. "free", "pro"
    LicenseKey    string    `db:"license_key"`     // Only populated if using offline keys
    ValidUntil    time.Time `db:"valid_until"`
    MaxApps       int       `db:"max_apps"`
    MaxThroughput int       `db:"max_throughput"`  // Msg/sec
}
```

---

## 3. UI Implementation Plan

### Organization Settings (New Tab)
1. Add a **"Billing & Licensing"** tab under Organization Settings.
2. **Pricing Component:** Show a simple pricing table (Free vs. Pro).
3. **Upgrade Flow:**
   - User clicks **Upgrade to Pro**.
   - UI calls `POST /v1/organizations/:org_id/billing/checkout`.
   - UI receives success response and displays a toast: `"Mock Upgrade Successful. Organization is now Pro."`
   - UI reloads the billing state and unlocks Pro features.

### Mid-Action Prompts (Upselling)
When a user on a free tier attempts to create an application that breaches the `MaxApps` limit, or selects a premium SMS provider:
- Intercept the action via a modal: *"Upgrade to Pro to create more applications."*
- Provide a direct link to the Organization Billing Tab.

---

## 4. Unifying "Personal Tooling" and "Organizations"

Currently, users have personal workspaces and orgs. To prevent duplication of the licensing logic:
- A user's "Personal Workspace" is just an internally managed `Tenant` mapping 1:1 to the User ID.
- Applying a license to a personal workspace utilizes the exact same `Tenant` licensing endpoints behind the scenes.

---

## 5. Security & Middleware

Update `internal/interfaces/http/middleware/license_check.go` to support this tiered hierarchy:
1. **Self-Hosted Mode:** Continues to check the environment `.env` or global Admin-level `license.key`.
2. **SaaS Mode:** Looks up `c.Locals("tenant").BillingTier`. If the endpoint requires "Pro" and the tenant is "Free", reject with `402 Payment Required`.

## 6. Future-Proofing (Phase 2 & Razorpay Integration)

While a specific payment gateway might change (Razorpay is the current frontrunner pending a cost-benefit analysis), we will abstract the payment provider to allow swapping easily.

### The PaymentProvider Interface
Introduce a `PaymentProvider` interface in `internal/domain/billing`:
```go
type PaymentProvider interface {
    CreateCheckoutSession(ctx context.Context, tenantID, tier string) (CheckoutResponse, error)
    VerifyWebhook(payload []byte, signature string) (WebhookEvent, error)
}
```

### Transition Steps
When transitioning from the "Mock" phase to actual payments (like Razorpay):
1. **Implement Razorpay Provider**: Create `internal/infrastructure/payment/razorpay.go` fulfilling `PaymentProvider`.
2. **Update Checkout Endpoint**: Change `POST .../checkout` to use the `PaymentProvider` to generate a Checkout Session URL (or order ID for Razorpay) and return it to the frontend to redirect the user or open the Razorpay widget.
3. **Implement Webhooks**: Implement a webhook endpoint like `POST /v1/webhooks/payment`. When the gateway sends a "payment completed" event, verify the signature, look up the `org_id` (tenant ID) from the metadata, and securely update the `BillingTier` to `pro` and extend `ValidUntil`.