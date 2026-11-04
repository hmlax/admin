# Go Admin

このブランチは **React + Hertz API + Kratos operator gRPC** を使用します。

```
React → admin HTTP :7788 → operator gRPC :7790 → データベース / Redis / オブジェクトストレージ
           SSE :7789 ← gRPC イベントストリーム ← Redis Pub/Sub
```

- フロントエンドは `frontend` のみです。React 19、Ant Design、TanStack Query を使用します。
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
pnpm dev
```

フロントエンドは `http://localhost:15888`。新規環境の初期アカウントは `admin / Abcd@1234` です。

デプロイ、Nacos、環境変数、移行の詳細は [サービス分割ガイド](docs/service_split.md) を参照してください。開発情報は [ドキュメント索引](docs/README.md) と [AGENTS.md](AGENTS.md) にあります。

## 検証

```sh
cd backend
go test ./...
go build -o operator/bin/operator ./operator/cmd/server
go build -o admin/bin/admin ./admin
```

React ディレクトリで `npm run typecheck` と `npm run build` を実行してください。
