# Go Admin

このブランチは **React + Hertz API + Kratos operator gRPC** を使用します。

```
React → admin HTTP :7788 → operator gRPC :7790 → データベース / Redis / オブジェクトストレージ
           SSE :7789 ← gRPC イベントストリーム ← Redis Pub/Sub
```

- `frontend/platform-admin` はプラットフォーム管理、`frontend/merchant-admin` は加盟店管理です。両 React アプリは独立してビルド・デプロイし、コンポーネントと API クライアントを共有します。
- `backend/admin`：HTTP、認証、RBAC、テナント入口検証、フィールド権限、監査。
- `backend/operator`：業務処理、GORM、ジョブ、データアクセス。
- `backend/platform`：共通依存、契約、生成コード、ビルド・デプロイツール。backend 直下のディレクトリはこの3つのみです。
- 環境変数は Nacos 接続設定のみ。アプリ設定は Nacos の server.yaml / data.yaml から読み込みます。

## ローカル起動

Go（バージョンは backend/go.mod）、Node.js、React ロックファイルに対応する pnpm を用意します。データベース、Redis、オブジェクトストレージを起動し、Nacos に server.yaml と data.yaml を登録してください。

```sh
cd backend
export NACOS_ADDR='http://127.0.0.1:8848'
export NACOS_GROUP='GO_ADMIN'
export NACOS_NAMESPACE_ID=''
export NACOS_USERNAME='your-nacos-user'
export NACOS_PASSWORD='your-nacos-password'
# ターミナル 1
go run ./operator/cmd/server
# ターミナル 2。同じ Nacos 接続設定を使用
go run ./admin
```

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm dev:platform
# 加盟店側：merchant-admin/.env.development.local.example を
# merchant-admin/.env.development.local にコピーし、既存の加盟店コードを設定
pnpm dev:merchant
```

プラットフォーム管理は `http://localhost:15888`、加盟店管理は `http://localhost:15889`。新規環境のプラットフォーム初期アカウントは `admin / Abcd@1234`、加盟店は各テナントのアカウントを使用します。両アプリは admin の HTTP/SSE にプロキシし、入口の権限はバックエンドが検証します。[フロントエンドの起動・デプロイ](frontend/README.md)を参照してください。

デプロイ、Nacos、環境変数、移行の詳細は [サービス分割ガイド](docs/service_split.md) を参照してください。開発情報は [ドキュメント索引](docs/README.md) と [AGENTS.md](AGENTS.md) にあります。

## 検証

```sh
cd backend
go test ./...
go build -o operator/bin/operator ./operator/cmd/server
go build -o admin/bin/admin ./admin
```

React ディレクトリで `npm run typecheck` と `npm run build` を実行してください。
