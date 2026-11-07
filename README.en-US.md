# Go Admin

A reusable administration framework with separate platform and merchant applications and tenant isolation. Includes authentication, administrators, roles, permissions, menus, API resources, organizations, audit logs, dictionaries, files and system settings.

React → Hertz HTTP admin → Kratos gRPC operator → GORM / database / Redis.

HTTP IDL: `backend/admin/idl/proto/admin/v1/`. RPC IDL: `backend/platform/proto/operator/v1/`. Generated RPC code: `backend/platform/pb/operator/`.

The merchant application uses a deployment-configured `X-Tenant-Code`; neither login form asks for it. AI, BI, notifications, scheduler, scripting, printing, monitoring and subscription billing are excluded. Existing database records are preserved.

See the [setup and validation commands](README.md), [frontend deployments](frontend/README.md) and [service guide](docs/service_split.md).
