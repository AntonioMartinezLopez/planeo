# Feature: Dynamic Organization ID Based on User Sub

## Overview
Replace hardcoded `organizationId: 1` in the frontend with dynamic organization fetching based on the authenticated user's OAuth "sub" claim.

## Progress Tracker

| Step | Description | Status | Notes |
|------|-------------|--------|-------|
| 1 | Review current organization schema and user-org relationship | ✅ Completed | |
| 2 | Create DTO for GetOrganizationByUserSub endpoint | ✅ Completed | Updated: no query param, reads from JWT |
| 3 | Add GetOrganizationByUserSub method to organization repository | ✅ Completed | |
| 4 | Add GetOrganizationByUserSub method to organization service | ✅ Completed | |
| 5 | Register GET endpoint in organization controller | ✅ Completed | Wired in app_factory.go |
| 6 | Test the new backend endpoint manually | ✅ Completed | Fixed middleware issue |
| 7 | Regenerate OpenAPI specs | ✅ Completed | |
| 8 | Regenerate frontend API client | ✅ Completed | |
| 9 | Extract sub field from auth session in frontend | ✅ Completed | |
| 10 | Create useOrganization composable | ✅ Completed | |
| 11 | Update index.vue to use dynamic organizationId | ✅ Completed | |
| 12 | Update usePaginatedRequests composable | ✅ Completed | Made organizationId reactive |
| 13 | End-to-end testing | ✅ Completed | Verified working in dev! |

## Files Created/Modified (Backend)

### New Files
- `services/core/internal/resources/organization/dto/organization.go` - Input/Output DTOs
- `services/core/internal/resources/organization/service.go` - Service layer
- `services/core/internal/resources/organization/controller.go` - HTTP controller

### Modified Files
- `libs/middlewares/claims.go` - Added `Sub` field to `OauthAccessClaims`
- `services/core/internal/resources/organization/repository.go` - Added `GetOrganizationsByUserSub`, fixed constructor name
- `services/core/internal/setup/app_factory.go` - Wired organization repository, service, controller
- `libs/middlewares/organization_validation.go` - Added early-return for routes without organizationId

### Test Files
- `services/core/internal/test/organization/organization_test.go` - Integration tests for GET /organizations endpoint

### Deleted Files
- `libs/middlewares/sub_validation.go` - No longer needed (reading sub from JWT directly)

## Frontend Implementation Summary

### New Composable: `useOrganization()`
Located at `web/app/composables/useOrganization.ts`:
- Fetches organizations for authenticated user (reads `sub` from session)
- Returns `organizations` (array), `organization` (first one), `organizationId` (computed)
- Uses TanStack Query with 5-minute cache

### Updated Components
1. **`index.vue`** - Now uses `useOrganization()` to get dynamic `organizationId`
2. **`usePaginatedRequests.ts`** - Updated to accept `Ref<number | null>` for reactive organizationId

---
*Last updated: ✅ FEATURE COMPLETE - All tests passing!*

## Summary

This feature successfully replaced the hardcoded `organizationId: 1` with dynamic organization fetching based on the authenticated user's OAuth `sub` claim.

### What Changed
- **Backend**: New `GET /organizations` endpoint that reads user's `sub` from JWT and returns their organizations
- **Frontend**: New `useOrganization()` composable that fetches and caches user's organizations
- **Integration**: All components now use dynamic organizationId instead of hardcoded values

### Production Ready
✅ Backend endpoint tested and working  
✅ Frontend composable tested and working  
✅ End-to-end flow verified in dev environment  
✅ Proper error handling and loading states  
✅ Caching implemented (5-minute stale time)

### Next Steps (Optional)
See **"Follow-up Tasks (After Feature Completion)"** section above for the middleware refactor recommendation.

## Step 1: Schema & Relationship Analysis

### Findings

**Database Tables (from migration `20241101135140_initialize_database.sql`):**

1. **`organizations`** table:
   - `id` (INTEGER, PK, auto-generated)
   - `name` (TEXT, unique)
   - `address` (TEXT)
   - `email` (TEXT)
   - `iam_organization_id` (TEXT) - links to Keycloak/IAM
   - `created_at`, `updated_at` (TIMESTAMPTZ)

2. **`users`** table:
   - `id` (INTEGER, PK, auto-generated)
   - `username`, `first_name`, `last_name`, `email` (TEXT)
   - `iam_user_id` (TEXT) - **This is the "sub" claim from OAuth!**
   - `organization_id` (INTEGER, FK → organizations.id)
   - `created_at`, `updated_at` (TIMESTAMPTZ)

### Key Relationships
```
users.organization_id → organizations.id (Many-to-One)
users.iam_user_id = OAuth "sub" claim
```

### Query Strategy
To get organization by user sub:
```sql
SELECT o.* 
FROM organizations o
JOIN users u ON u.organization_id = o.id
WHERE u.iam_user_id = @userSub
LIMIT 1;
```

### Existing Code Structure

- **Organization Model** (`models/organization.go`): ✅ Already has all fields
- **Organization Repository** (`repository.go`): Has `GetOrganizationIamById` - needs new method
- **Organization Controller**: ❌ Does NOT exist - needs to be created
- **Organization Service**: ❌ Does NOT exist - needs to be created
- **Organization DTO folder**: ❌ Does NOT exist - needs to be created

### Decision Points
1. Should we create a full service layer or just add the repository method and use it directly in controller?
   - **Recommendation**: Create service layer for consistency with other resources
2. Should the endpoint be `/organizations/by-user?userSub=xxx` or `/organizations/me` (using JWT claims)?
   - **Recommendation**: Use `/organizations/by-user?userSub=xxx` as specified, but validate against JWT

---
*Last updated: Step 1 completed ✅*

## User Decisions (from discussion)

1. **Validation**: ~~Yes, validate that `sub` query param matches JWT's `sub` claim~~ **UPDATED**: Read `sub` directly from JWT claims (no query param needed)
2. **Response format**: Return an **array** of organizations (future-proofing for multi-org)
3. **Endpoint path**: ~~`GET /organizations?sub=xxx`~~ **UPDATED**: `GET /organizations` (reads sub from JWT automatically)

## Step 1.5: Validation Approach Analysis

### ~~Original Approach~~ (Deprecated)
Was going to use query param + validation middleware. 

### **New Approach** (Simpler & More Secure)
Read `sub` directly from JWT claims in the controller handler:
```go
claims := ctx.Context().Value(middlewares.AccessClaimsContextKey{}).(*middlewares.OauthAccessClaims)
userSub := claims.Sub
// Use userSub to query organizations
```

**Benefits:**
- No query parameter needed
- Cannot be spoofed
- Simpler API: `GET /organizations`
- No need for `sub_validation` middleware

**Files to clean up:**
- `libs/middlewares/sub_validation.go` - Can be deleted (won't be used)

---

## Follow-up Tasks (After Feature Completion)

### Refactor: Route-Specific Organization Middleware

**Issue discovered during Step 6:**
The `OrganizationCheckMiddleware` is applied globally to all routes, but the new `GET /organizations` endpoint doesn't have an `organizationId` path parameter. This caused an error: `invalid input syntax for type integer: ""`.

**Current fix (quick):**
Added early-return in `libs/middlewares/organization_validation.go` to skip routes without `organizationId`:
```go
organizationId := ctx.Param("organizationId")
if organizationId == "" {
    next(ctx)
    return
}
```

**Recommended refactor:**
Convert `OrganizationCheckMiddleware` to a route-specific middleware (like `permissions.Apply()`):

1. **Remove global middleware** from `app_factory.go`:
   ```go
   // Remove from global middlewares array
   middlewares := []api.Middleware{
       middlewares.AuthMiddleware(...),
       // middlewares.OrganizationCheckMiddleware(...), // REMOVE
   }
   ```

2. **Create wrapper pattern** in `libs/middlewares/organization_validation.go`:
   ```go
   type OrganizationMiddlewareConfig struct {
       api               huma.API
       resolveOrganization func(string) (string, error)
   }
   
   func NewOrganizationMiddlewareConfig(api huma.API, resolver func(string) (string, error)) *OrganizationMiddlewareConfig {
       return &OrganizationMiddlewareConfig{api: api, resolveOrganization: resolver}
   }
   
   func (c *OrganizationMiddlewareConfig) Apply() func(huma.Context, func(huma.Context)) {
       // Return the middleware function
   }
   ```

3. **Apply per-route** in controllers:
   ```go
   huma.Register(c.api, humaUtils.WithAuth(huma.Operation{
       Middlewares: huma.Middlewares{
           orgCheck.Apply(),  // Only on routes with {organizationId}
           permissions.Apply("category", "read"),
       },
   }), handler)
   ```

**Files to modify:**
- `libs/middlewares/organization_validation.go` - Refactor to config/Apply pattern
- `services/core/internal/setup/app_factory.go` - Remove global middleware, pass config to controllers
- `services/core/internal/resources/category/controller.go` - Add middleware per-route
- `services/core/internal/resources/request/controller.go` - Add middleware per-route
- `services/core/internal/resources/user/controller.go` - Add middleware per-route

**Effort:** ~30-45 minutes

**Priority:** Low (current fix works, refactor for cleanliness)

---
---
*Last updated: Approach changed - reading sub from JWT directly*

