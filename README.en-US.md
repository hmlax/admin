# Go Admin

This branch uses **React + Hertz API + Kratos operator gRPC**.

```
React → admin HTTP :7788 → operator gRPC :7790 → Database / Redis / Object storage
           SSE :7789 ← gRPC event stream ← Redis Pub/Sub
```

- The only frontend is `frontend`, with React 19, Ant Design and TanStack Query.
- `backend/admin`: HTTP, authentication, RBAC, tenant entry checks, field permissions and auditing.
- `backend/operator`: business operations, GORM, jobs and data access.
- `backend/platform`: shared dependencies, contracts, generated code and build/deploy tools; these are the only three top-level backend directories.
- Nacos provides configuration and service discovery; env contains only Nacos connection settings, while server.yaml/data.yaml provide application settings.

## Local startup

Install Go (version in backend/go.mod), Node.js and pnpm matching the React lockfile. Start the database, Redis and object storage, then publish server.yaml and data.yaml in Nacos.

```sh
cd backend
export NACOS_ADDR='http://127.0.0.1:8848'
export NACOS_GROUP='GO_ADMIN'
export NACOS_NAMESPACE_ID=''
export NACOS_USERNAME='your-nacos-user'
export NACOS_PASSWORD='your-nacos-password'
# Terminal one
go run ./operator/cmd/server
# Terminal two, with the same Nacos environment settings
go run ./admin
```

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm dev
```

The frontend runs at `http://localhost:15888`. A fresh installation seeds `admin / Abcd@1234`.

See the [service split guide](docs/service_split.md) for deployment, Nacos, environment variables and migration. See the [documentation index](docs/README.md) and [AGENTS.md](AGENTS.md) for development.

## Verification

```sh
cd backend
go test ./...
go build -o operator/bin/operator ./operator/cmd/server
go build -o admin/bin/admin ./admin
```

Run `npm run typecheck` and `npm run build` in the React directory.
