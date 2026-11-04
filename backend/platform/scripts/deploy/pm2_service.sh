#!/usr/bin/env bash

set -euo pipefail

if command -v go >/dev/null 2>&1; then
  echo "go: $(go version)"
else
  echo "go: not installed"
fi

if command -v pm2 >/dev/null 2>&1; then
  echo "pm2: $(pm2 -v)"
else
  echo "pm2: not installed"
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
project_root="$script_dir/../../.."
env_file="$project_root/.env"
project_root="$(cd "$project_root" && pwd)"

err() { printf '%s\n' "$*" >&2; }

# 加载 .env（去掉 CRLF），仅在文件存在且非空时加载
if [ -f "$env_file" ] && [ -s "$env_file" ]; then
  # 自动导出变量到环境，便于 make/go 使用
  set -a
  # shellcheck disable=SC1090
  source <(sed 's/\r$//' "$env_file")
  set +a
else
  err "no .env found at $env_file, continuing without it"
fi

# 进入项目根以保证相对路径一致
pushd "$project_root" >/dev/null

# 构建项目
if ! make build_only; then
  err "make build failed"
  popd >/dev/null
  exit 1
fi

popd >/dev/null

project_name="${PROJECT_NAME:-gwa}"
install_root="${HOME}/app/${project_name}"
app_root="${project_root}"

# 两个应用直接位于 backend 下。
# 使用兼容的方式替代 mapfile（支持 macOS bash 等旧版本）
apps=()
while IFS= read -r -d '' app_dir; do
  apps+=("$app_dir")
done < <(printf "%s\0" "$app_root/operator" "$app_root/admin")

# 如果没有服务，直接退出
if [ "${#apps[@]}" -eq 0 ]; then
  err "no apps found under $app_root"
  exit 0
fi

mkdir -p "$install_root"

for app_dir in "${apps[@]}"; do
  app="$(basename "$app_dir")"
  echo "Installing service: $app"

  app_install_root="$install_root/$app"
  bin_src="$app_dir/bin/$app"
  bin_dest="$app_install_root/bin"

  mkdir -p "$bin_dest"

  # 复制二进制（如果存在）
  if [ -f "$bin_src" ]; then
    cp -f "$bin_src" "$bin_dest/server"
    chmod +x "$bin_dest/server" || true
  else
    err "binary not found: $bin_src (skipping)"
    continue
  fi


done

# 启动/注册到 PM2
for app_dir in "${apps[@]}"; do
  app="$(basename "$app_dir")"
  full_app_name="${project_name}-${app}"
  echo "Starting service: $app"

  app_install_root="$install_root/$app"
  bin_path="$app_install_root/bin/server"

  if [ ! -x "$bin_path" ]; then
    err "executable not found or not executable: $bin_path (skipping)"
    continue
  fi

  # 使用绝对二进制路径启动；配置由环境变量提供
  # pm2 usage: pm2 start <scripts> --name <name> --namespace <ns> -- <args...>
  if ! command -v pm2 >/dev/null 2>&1; then
    err "pm2 not installed or not in PATH; please install pm2"
    exit 1
  fi

  pushd "$app_install_root/bin" >/dev/null

  pm2 delete "$full_app_name" --namespace "$project_name" >/dev/null 2>&1 || true

  pm2 start "$bin_path" \
    --name "$full_app_name" \
    --namespace "$project_name" \
    --cwd "$app_install_root/bin" \
    --output "$app_install_root/bin/stdout.log" \
    --error "$app_install_root/bin/stderr.log" \
    --update-env

  popd >/dev/null
done

pm2 save

echo "install and pm2 setup complete for namespace: $project_name"
