# Go Admin

プラットフォーム管理画面と加盟店管理画面を個別に構築・配備できる汎用管理基盤です。認証、管理者、ロール、権限、メニュー、API、組織、監査ログ、辞書、ファイル、システム設定とテナント分離を備えています。

React → Hertz HTTP admin → Kratos gRPC operator → GORM / database / Redis.

HTTP IDL: `backend/admin/idl/proto/admin/v1/`. RPC IDL: `backend/platform/proto/operator/v1/`. Generated RPC code: `backend/platform/pb/operator/`.

The merchant application uses a deployment-configured `X-Tenant-Code`; neither login form asks for it. AI, BI, notifications, scheduler, scripting, printing, monitoring and subscription billing are excluded. Existing database records are preserved.

See the [setup and validation commands](README.md), [frontend deployments](frontend/README.md) and [service guide](docs/service_split.md).
