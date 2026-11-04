"""HTTP tenant onboarding regression against the running infra-docker stack.

Business changes use public APIs. The only test-only infrastructure read obtains
our own newly generated CAPTCHA from Redis; authentication and RBAC stay enabled.
"""

from contextlib import contextmanager
from datetime import datetime, timedelta, timezone
import http.cookiejar
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import time
import traceback
import urllib.error
import urllib.parse
import urllib.request

try:
    import yaml
except ImportError as error:
    raise SystemExit("Install test dependencies: python3 -m pip install -r platform/scripts/test/requirements.txt") from error


ROOT = Path(__file__).resolve().parents[3]
MODULES = ("DASHBOARD", "OPM", "SYSTEM", "DICT", "PERMISSION", "LOG",
           "INTERNAL_MESSAGE", "FILE", "TASK", "TENANT", "AI")


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def timestamp(days=0):
    return (datetime.now(timezone.utc) + timedelta(days=days)).isoformat().replace("+00:00", "Z")


def password():
    return "Qa7!" + secrets.token_urlsafe(15)


def private_json(path, value):
    # Create the file with restrictive permissions before writing credentials.
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(descriptor, "w") as stream:
        json.dump(value, stream, ensure_ascii=False, indent=2)
        stream.write("\n")


class Report:
    def __init__(self, output_root=None, title="租户生命周期自动化测试"):
        self.title = title
        self.run_id = datetime.now(timezone.utc).strftime("%Y%m%d%H%M%S") + "_" + secrets.token_hex(3)
        self.output_root = Path(output_root) if output_root else ROOT / "platform/artifacts" / "tenant-e2e"
        self.directory = self.output_root / self.run_id
        self.directory.mkdir(parents=True, mode=0o700)
        self.steps = []
        self.secrets = []
        self.resources = {"plans": [], "tenants": []}
        self.active = None
        self.ready = False
        self.failure_trace = None

    def redact(self, value):
        text = str(value)
        for secret in sorted(self.secrets, key=len, reverse=True):
            if secret:
                text = text.replace(secret, "[REDACTED]")
        return text

    @contextmanager
    def step(self, name):
        result = {"name": name, "status": "RUNNING", "requests": []}
        self.steps.append(result)
        self.active = result
        start = time.monotonic()
        try:
            yield
            result["status"] = "PASS"
        except Exception as error:
            result["status"] = "FAIL"
            result["error"] = self.redact(error)
            raise
        finally:
            result["duration_ms"] = round((time.monotonic() - start) * 1000)
            self.active = None
            print(result["status"] + " " + name, flush=True)

    def save(self, base, credentials, failure=None):
        passed = sum(item["status"] == "PASS" for item in self.steps)
        result = {
            "run_id": self.run_id, "finished_at": timestamp(), "base_url": base,
            "status": "FAIL" if failure else "PASS", "passed": passed,
            "total": len(self.steps), "failure": self.redact(failure) if failure else None,
            "resources": self.resources, "steps": self.steps,
            "failure_trace": self.failure_trace,
            "http_requests": sum(len(item["requests"]) for item in self.steps),
            "expected_rejections": sum(request["status"] >= 400 and request["status"] in request["expected_statuses"]
                                       for item in self.steps for request in item["requests"]),
        }
        private_json(self.directory / "report.json", result)
        private_json(self.directory / "credentials.json", credentials)
        private_json(self.output_root / "latest.json", {
            "run_id": self.run_id, "status": result["status"], "ready": credentials.get("ready", False),
            "report": str(self.directory / "report.md"), "credentials": str(self.directory / "credentials.json"),
        })
        lines = ["# " + self.title, "", "- 运行：`" + self.run_id + "`",
                 "- 地址：" + base, "- 结果：" + result["status"],
                 "- 通过：" + str(passed) + "/" + str(len(self.steps)),
                 "- HTTP 请求：" + str(result["http_requests"]), "",
                 "凭据单独保存在同目录 credentials.json（权限 0600，已被 Git 和 Docker 忽略）。", "",
                 "| 测试 | 结果 | 耗时 ms |", "|---|---|---|"]
        for item in self.steps:
            lines.append("| " + item["name"] + " | " + item["status"] + " | " + str(item["duration_ms"]) + " |")
        if failure:
            lines.extend(["", "失败原因：" + self.redact(failure), "", "当次资源保留供排查；流程以非零状态退出。"])
        for item in self.resources.get("external_integrations", []):
            lines.extend(["", "未覆盖：" + item["feature"] + "；原因：" + item["reason"] + "。"])
        (self.directory / "report.md").write_text("\n".join(lines) + "\n")
        print("Report: " + str(self.directory / "report.md"))
        print("Credentials: " + str(self.directory / "credentials.json"))


class Infrastructure:
    def __init__(self, report):
        compose = ["docker", "compose", "--env-file", str(ROOT / ".env.infra"),
                   "-f", str(ROOT / "docker-compose.infra.yaml"), "config", "--format", "json"]
        config = json.loads(subprocess.check_output(compose, stderr=subprocess.PIPE))
        env = config["services"]["admin"]["environment"]
        report.secrets.extend([env["NACOS_PASSWORD"]])
        nacos = os.environ.get("NACOS_PUBLISH_URL", "http://127.0.0.1:8848").rstrip("/")
        login = urllib.request.Request(nacos + "/nacos/v1/auth/users/login", data=urllib.parse.urlencode({
            "username": env["NACOS_USERNAME"], "password": env["NACOS_PASSWORD"],
        }).encode())
        access_token = ""
        try:
            with urllib.request.urlopen(login, timeout=10) as response:
                access_token = json.load(response)["accessToken"]
            query = urllib.parse.urlencode({
                "dataId": "data.yaml", "group": env["NACOS_GROUP"],
                "tenant": env.get("NACOS_NAMESPACE_ID", ""), "accessToken": access_token,
            })
            with urllib.request.urlopen(nacos + "/nacos/v1/cs/configs?" + query, timeout=10) as response:
                document = yaml.safe_load(response.read())
        except urllib.error.HTTPError as error:
            # Nacos URLs contain access tokens; omit the URL from diagnostics.
            report.secrets.append(access_token)
            raise RuntimeError("Nacos bootstrap failed (HTTP " + str(error.code) + ")") from error
        self.redis = document["data"]["redis"]
        self.minio = document.get("oss", {}).get("minio", {})
        self.key = document["runtime"]["crypto_key"].encode().hex()
        report.secrets.extend([self.redis.get("password", ""), document["runtime"]["crypto_key"], self.key])

    def encrypt(self, plain):
        return subprocess.check_output([
            "openssl", "enc", "-aes-128-cbc", "-K", self.key, "-iv", self.key, "-base64", "-A",
        ], input=plain.encode(), stderr=subprocess.PIPE).decode()

    def captcha(self, client):
        item = client.request("GET", "/admin/v1/captcha", authenticated=False)
        answer = subprocess.check_output([
            "docker", "exec", "-e", "REDISCLI_AUTH=" + self.redis.get("password", ""),
            os.environ.get("E2E_REDIS_CONTAINER", "infra-redis"), "redis-cli", "-n",
            str(self.redis.get("db", 0)), "--raw", "GET", "go-admin:captcha:" + item["captchaId"],
        ], text=True, stderr=subprocess.PIPE).strip()
        require(bool(answer), "new CAPTCHA was not found in the configured test Redis DB")
        return {"X-Captcha-Id": item["captchaId"], "X-Captcha-Value": answer.lower()}


class Client:
    def __init__(self, base, report, infra=None):
        self.base, self.report, self.infra = base, report, infra
        self.cookies = http.cookiejar.CookieJar()
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.cookies))
        self.token = None
        self.tenant_code = ""

    def wait_ready(self):
        deadline = time.monotonic() + 30
        last_error = None
        while time.monotonic() < deadline:
            try:
                with self.opener.open(self.base + "/admin/v1/sso/login-info", timeout=3) as response:
                    require(response.status == 200, "unexpected readiness response")
                    return
            except urllib.error.HTTPError as error:
                if error.code not in (500, 502, 503, 504):
                    raise
                last_error = error
            except (urllib.error.URLError, ConnectionError) as error:
                last_error = error
            # Only read-only startup probes retry; business writes never retry.
            time.sleep(0.25)
        raise RuntimeError("API/operator did not become ready within 30 seconds") from last_error

    def request(self, method, path, body=None, expected=200, reason=None, message=None,
                authenticated=True, headers=None, query=None, binary=False):
        values = {"Content-Type": "application/json", "Accept-Language": "zh-CN",
                  "User-Agent": "GoAdminMerchantQA/1.0"}
        if self.tenant_code:
            values["X-Tenant-Code"] = self.tenant_code
        if authenticated and self.token:
            values["Authorization"] = "Bearer " + self.token
        values.update(headers or {})
        # Platform proxies deliberately strip merchant selection. Multi-tenant
        # isolation tests use the real gateway; a single-merchant run may use its frontend.
        base = os.environ.get("E2E_TENANT_BASE_URL", "http://localhost:7788").rstrip("/") if self.tenant_code else self.base
        url = base + path
        if query:
            url += "?" + urllib.parse.urlencode(query)
        data = None if body is None else json.dumps(body).encode()
        req = urllib.request.Request(url, data=data, method=method, headers=values)
        start = time.monotonic()
        try:
            with self.opener.open(req, timeout=15) as response:
                status, raw = response.status, response.read()
        except urllib.error.HTTPError as error:
            status, raw = error.code, error.read()
        allowed = (expected,) if isinstance(expected, int) else expected
        record = {
            "method": method, "path": path, "status": status,
            "duration_ms": round((time.monotonic() - start) * 1000),
            "reason": None, "expected_statuses": list(allowed),
        }
        if self.report.active is not None:
            self.report.active["requests"].append(record)
        if binary:
            require(status in allowed, method + " " + path + ": HTTP " + str(status) + ", expected " + str(allowed))
            return raw
        try:
            payload = json.loads(raw) if raw else {}
        except json.JSONDecodeError as error:
            raise AssertionError(method + " " + path + ": response is not one JSON document (HTTP " + str(status) + ")") from error
        require(isinstance(payload, dict), path + ": expected a JSON object")
        record["reason"] = payload.get("reason")
        require(status in allowed, method + " " + path + ": HTTP " + str(status) +
                ", expected " + str(allowed) + "; " + payload.get("reason", "") + " " + payload.get("message", ""))
        if reason:
            require(payload.get("reason") == reason, path + ": unexpected error reason " + str(payload.get("reason")))
        if message:
            require(message in payload.get("message", ""), path + ": unexpected error message " + str(payload.get("message")))
        return payload

    def login(self, username, plain, tenant_code="", expected=200, reason=None, captcha=None):
        captcha_headers = captcha if captcha is not None else self.infra.captcha(self)
        self.tenant_code = tenant_code
        payload = self.request("POST", "/admin/v1/login", {
            "grant_type": "password", "username": username,
            "password": self.infra.encrypt(plain),
        }, expected=expected, reason=reason, authenticated=False,
            headers=captcha_headers)
        if expected == 200:
            self.token = payload.get("access_token")
            require(bool(self.token), "login did not return an access token")
            self.report.secrets.append(self.token)
            cookie_name = "refresh_token" + ("_" + tenant_code if tenant_code else "")
            require(any(cookie.name == cookie_name and cookie.has_nonstandard_attr("HttpOnly")
                        for cookie in self.cookies), "login did not issue an HttpOnly refresh cookie")
        return payload

    def find(self, resource, field, value, filters=None):
        query = dict(filters or {})
        query[field] = value
        payload = self.request("GET", "/admin/v1/" + resource, query={
            "query": json.dumps(query), "noPaging": "true",
        })
        matches = [item for item in payload.get("items", []) if item.get(field) == value]
        require(len(matches) == 1, resource + ": expected exactly one matching " + field)
        return matches[0]

    def update(self, resource, identifier, data):
        return self.request("PUT", "/admin/v1/" + resource + "/" + str(identifier), {
            "data": data, "updateMask": ",".join(data),
        })


def ordered_menus(items):
    result = []
    for item in items:
        node = dict(item)
        if "children" in node:
            node["children"] = ordered_menus(node["children"])
        result.append(node)
    return sorted(result, key=lambda node: (node.get("name", ""), node.get("path", "")))


def menu_nodes(items):
    for item in items:
        yield item
        yield from menu_nodes(item.get("children", []))


class Lifecycle:
    def __init__(self, report):
        self.report = report
        self.base = os.environ.get("E2E_BASE_URL", "http://localhost:15888").rstrip("/")
        self.primary = None
        self.infra = None
        self.admin = None
        self.plan_id = None
        self.quota_ids = {}
        self.module_ids = {}
        self.username = "owner_" + report.run_id[-6:]
        self.initial_password, self.final_password, self.other_password = password(), password(), password()
        self.report.secrets.extend([self.initial_password, self.final_password, self.other_password])
        self.current_password = self.initial_password
        self.tenants = []

    def new_client(self):
        return Client(self.base, self.report, self.infra)

    def create_plan(self, suffix):
        name = "E2E_" + self.report.run_id + "_" + suffix
        self.admin.request("POST", "/admin/v1/plans", {"data": {
            "name": name, "version": "STANDARD", "expiryPolicy": "READONLY",
            "dataRetentionDays": 30, "description": "自动化租户生命周期测试",
        }})
        plan = self.admin.find("plans", "name", name)
        self.report.resources["plans"].append({"id": plan["id"], "name": name, "retained": True})
        return plan

    def create_module(self, module):
        self.admin.request("POST", "/admin/v1/plan-modules", {"data": {"planId": self.plan_id, "module": module}})
        item = self.admin.find("plan-modules", "module", module, {"plan_id": self.plan_id})
        self.module_ids[module] = item["id"]
        return item

    def create_tenant(self, suffix, plain, with_plan=True):
        code = "e2e_" + self.report.run_id + "_" + suffix
        tenant = {"name": "自动化租户_" + code, "code": code, "type": "TRIAL", "status": "ON",
                  "auditStatus": "APPROVED", "expiredAt": timestamp(30)}
        if with_plan:
            tenant["planId"] = self.plan_id
        body = {"tenant": tenant, "user": {"username": self.username}, "password": plain}
        self.admin.request("POST", "/admin/v1/tenants:with-admin", body)
        row = self.admin.find("tenants", "code", code)
        self.tenants.append(row)
        self.report.resources["tenants"].append({"id": row["id"], "code": code, "retained": True})
        return row, body

    def assert_profile(self, client, tenant):
        me = client.request("GET", "/admin/v1/me")
        require(me.get("tenantId") == tenant["id"] and me.get("username") == self.username,
                "profile crossed a tenant boundary")
        require(me.get("id") == tenant["adminUserId"], "tenant administrator link does not match login identity")
        return me

    def assert_member_creation_restored(self, client, prefix):
        name = prefix + "_" + self.report.run_id[-6:]
        client.request("POST", "/admin/v1/users", {
            "data": {"username": name, "roleIds": self.role_ids}, "password": password(),
        })
        member = client.find("users", "username", name)
        require(member["tenantId"] == self.primary["id"], "restored member creation crossed tenant boundary")
        client.request("DELETE", "/admin/v1/users/" + str(member["id"]))

    def run(self):
        step = self.report.step
        with step("基础设施配置、平台登录和未认证请求门禁"):
            self.infra = Infrastructure(self.report)
            self.admin = self.new_client()
            self.admin.wait_ready()
            platform_password = os.environ.get("ADMIN_SMOKE_PASSWORD", "Abcd@1234")
            self.report.secrets.append(platform_password)
            self.admin.request("GET", "/admin/v1/me", authenticated=False, expected=401)
            self.admin.login(os.environ.get("ADMIN_SMOKE_USERNAME", "admin"), platform_password)
            require(self.admin.request("GET", "/admin/v1/me").get("tenantId", 0) == 0,
                    "provisioning account must be a platform account")

        with step("创建套餐、查询详情及更新套餐"):
            plan = self.create_plan("demo")
            self.plan_id = plan["id"]
            self.admin.update("plans", self.plan_id, {"description": "自动化验收套餐：保留供登录体验"})
            require(self.admin.request("GET", "/admin/v1/plans/" + str(self.plan_id))["version"] == "STANDARD",
                    "plan version was not persisted")

        with step("创建套餐全部模块白名单并查询详情"):
            for module in MODULES:
                item = self.create_module(module)
                require(self.admin.request("GET", "/admin/v1/plan-modules/" + str(item["id"]))["module"] == module,
                        "plan module was not persisted")

        with step("四类套餐配额创建、查询及更新"):
            for kind, value in {"USER_LIMIT": 20, "STORAGE": 1073741824, "API_CALL": 100000, "AI_TOKENS": 100000}.items():
                self.admin.request("POST", "/admin/v1/plan-quotas", {"data": {
                    "planId": self.plan_id, "quotaType": kind, "quotaValue": str(value),
                }})
                item = self.admin.find("plan-quotas", "quotaType", kind, {"plan_id": self.plan_id})
                self.quota_ids[kind] = item["id"]
                self.admin.update("plan-quotas", item["id"], {"quotaValue": str(value + 1)})
                updated = self.admin.find("plan-quotas", "quotaType", kind, {"plan_id": self.plan_id})
                require(int(updated["quotaValue"]) == value + 1, "quota update did not persist")
                self.admin.update("plan-quotas", item["id"], {"quotaValue": str(value)})

        with step("创建已绑定套餐的租户与管理员，确认角色初始化"):
            self.primary, body = self.create_tenant("demo", self.initial_password)
            require(self.primary.get("planId") == self.plan_id and self.primary.get("adminUserId"),
                    "tenant plan/admin association was not created")
            user = self.admin.request("GET", "/admin/v1/users/" + str(self.primary["adminUserId"]))
            require(user.get("tenantId") == self.primary["id"] and user.get("roleIds"),
                    "tenant administrator role was not initialized")
            self.role_ids = user["roleIds"]

        with step("重复租户编号被拒，原租户保持唯一"):
            self.admin.request("POST", "/admin/v1/tenants:with-admin", body, expected=400,
                               message="already exists")
            self.admin.find("tenants", "code", self.primary["code"])

        with step("弱密码拒绝创建并回滚租户、用户和角色"):
            invalid_code = "e2e_" + self.report.run_id + "_weak"
            before_users = self.admin.request("GET", "/admin/v1/users", query={"noPaging": "true"}).get("items", [])
            before_roles = self.admin.request("GET", "/admin/v1/roles", query={"noPaging": "true"}).get("items", [])
            self.admin.request("POST", "/admin/v1/tenants:with-admin", {
                "tenant": {"name": invalid_code, "code": invalid_code, "status": "ON"},
                "user": {"username": self.username}, "password": "123",
            }, expected=400)
            exists = self.admin.request("GET", "/admin/v1/tenants:exists", query={"code": invalid_code})
            if exists.get("exist"):
                orphan = self.admin.find("tenants", "code", invalid_code)
                self.report.resources["tenants"].append({"id": orphan["id"], "code": invalid_code, "retained": True})
            require(not exists.get("exist"), "failed tenant creation was not rolled back")
            after_users = self.admin.request("GET", "/admin/v1/users", query={"noPaging": "true"}).get("items", [])
            after_roles = self.admin.request("GET", "/admin/v1/roles", query={"noPaging": "true"}).get("items", [])
            require({row["id"] for row in before_users} == {row["id"] for row in after_users}, "failed creation left an orphan user")
            require({row["id"] for row in before_roles} == {row["id"] for row in after_roles}, "failed creation left an orphan role")

        with step("验证码必填、错误密码和错误租户均被拒"):
            self.new_client().login(self.username, self.initial_password, self.primary["code"],
                                    expected=400, captcha={})
            self.new_client().login(self.username, password(), self.primary["code"],
                                    expected=400, reason="INVALID_PASSWORD")
            self.new_client().login(self.username, self.initial_password, "missing_" + self.report.run_id, expected=403)

        tenant = self.new_client()
        with step("小写验证码登录、HttpOnly Cookie 与验证码一次性消费"):
            captcha = self.infra.captcha(tenant)
            tenant.login(self.username, self.initial_password, self.primary["code"], captcha=captcha)
            self.assert_profile(tenant, self.primary)
            self.new_client().login(self.username, self.initial_password, self.primary["code"],
                                    expected=400, captcha=captcha)

        with step("用户资料、权限码、菜单与聚合初始化上下文一致"):
            self.assert_profile(tenant, self.primary)
            permissions = tenant.request("GET", "/admin/v1/perm-codes")
            routes = tenant.request("GET", "/admin/v1/routes")
            initial = tenant.request("GET", "/admin/v1/initial-context")
            require("sys:access_backend" in permissions.get("codes", []), "base backend permission missing")
            require(initial.get("permissions", []) == permissions.get("codes", []), "initial permissions mismatch")
            require(initial.get("hiddenFields", []) == permissions.get("hiddenFields", []), "hidden fields mismatch")
            require(ordered_menus(initial.get("menus", [])) == ordered_menus(routes.get("items", [])), "initial menu mismatch")
            require(bool(routes.get("items")), "new tenant did not receive any navigation")

        with step("租户身份不可访问平台专属配置接口"):
            tenant.request("GET", "/admin/v1/notification-channels", expected=403)

        with step("Cookie 刷新会话后继续访问当前租户"):
            refreshed = tenant.request("POST", "/admin/v1/refresh-token", {"grant_type": "refresh_token"}, authenticated=False)
            tenant.token = refreshed["access_token"]
            self.report.secrets.append(tenant.token)
            self.assert_profile(tenant, self.primary)

        with step("SSE 建连并接收连接帧"):
            stream_base = os.environ.get("E2E_SSE_URL", "http://localhost:7789").rstrip("/")
            req = urllib.request.Request(stream_base + "/events?stream=" + str(self.primary["adminUserId"]),
                                         headers={"Authorization": "Bearer " + tenant.token,
                                                  "X-Tenant-Code": self.primary["code"]})
            with urllib.request.urlopen(req, timeout=10) as response:
                require(response.status == 200, "SSE handshake did not return HTTP 200")
                require(response.headers.get_content_type() == "text/event-stream", "SSE response content type mismatch")
                require(response.readline().strip() == b": connected", "SSE connection frame missing")
                self.report.active["requests"].append({
                    "method": "GET", "path": "/events", "status": 200,
                    "expected_statuses": [200], "reason": None,
                })

        with step("新租户无套餐时拒绝访问，绑定套餐后恢复"):
            other, _ = self.create_tenant("isolation", self.other_password, with_plan=False)
            other_client = self.new_client()
            other_client.login(self.username, self.other_password, other["code"])
            other_client.request("GET", "/admin/v1/me", expected=403, message="module not allowed")
            self.admin.update("tenants", other["id"], {"planId": self.plan_id})
            self.assert_profile(other_client, other)

        with step("同名管理员按租户验证密码，同名角色同时可用"):
            self.new_client().login(self.username, self.initial_password, other["code"], expected=400, reason="INVALID_PASSWORD")
            self.assert_profile(tenant, self.primary)
            self.assert_profile(other_client, other)

        with step("用户列表仅含当前租户，跨租户读取及修改被拒"):
            for current, own, foreign in [(tenant, self.primary, other), (other_client, other, self.primary)]:
                own_roles = self.assert_profile(current, own)["roleIds"]
                users = current.request("GET", "/admin/v1/users", query={"noPaging": "true"}).get("items", [])
                require(users and all(row.get("tenantId") == own["id"] for row in users), "user list exposed another tenant")
                current.request("GET", "/admin/v1/users/" + str(foreign["adminUserId"]), expected=(403, 404))
                current.request("PUT", "/admin/v1/users/" + str(foreign["adminUserId"]), {
                    "data": {"nickname": "forbidden-change", "roleIds": own_roles}, "updateMask": "nickname",
                }, expected=(403, 404))
                foreign_user = self.admin.request("GET", "/admin/v1/users/" + str(foreign["adminUserId"]))
                require(foreign_user.get("nickname") != "forbidden-change", "cross-tenant update changed target")

        with step("租户成员 CRUD，伪造 tenantId 仍归属当前租户"):
            member_name = "member_" + self.report.run_id[-6:]
            tenant.request("POST", "/admin/v1/users", {"data": {
                "username": member_name, "tenantId": other["id"], "status": "NORMAL", "roleIds": self.role_ids,
            }, "password": password()})
            member = tenant.find("users", "username", member_name)
            require(member["tenantId"] == self.primary["id"], "caller-controlled tenantId crossed tenant boundary")
            tenant.request("PUT", "/admin/v1/users/" + str(member["id"]), {
                "data": {"nickname": "自动化成员", "roleIds": self.role_ids}, "updateMask": "nickname",
            })
            require(tenant.request("GET", "/admin/v1/users/" + str(member["id"]))["nickname"] == "自动化成员",
                    "member update failed")
            tenant.request("DELETE", "/admin/v1/users/" + str(member["id"]))

        with step("用户配额到限拒绝创建，提高配额恢复"):
            self.admin.update("plan-quotas", self.quota_ids["USER_LIMIT"], {"quotaValue": "1"})
            try:
                tenant.request("POST", "/admin/v1/users", {"data": {"username": "quota_" + self.report.run_id[-6:], "roleIds": self.role_ids},
                                                           "password": password()}, expected=403, message="quota")
            finally:
                self.admin.update("plan-quotas", self.quota_ids["USER_LIMIT"], {"quotaValue": "20"})
            self.assert_member_creation_restored(tenant, "quota_restored")

        with step("API 调用配额到限拒绝访问，提高配额恢复"):
            self.admin.update("plan-quotas", self.quota_ids["API_CALL"], {"quotaValue": "0"})
            try:
                tenant.request("GET", "/admin/v1/me", expected=403, message="API_CALL")
            finally:
                self.admin.update("plan-quotas", self.quota_ids["API_CALL"], {"quotaValue": "100000"})
            self.assert_profile(tenant, self.primary)

        with step("移除模块阻断相关接口，恢复白名单后可用"):
            before = tenant.request("GET", "/admin/v1/routes").get("items", [])
            require(any(node.get("component", "").startswith("app/opm/") for node in menu_nodes(before)),
                    "test tenant must initially have an authorized OPM menu")
            self.admin.request("DELETE", "/admin/v1/plan-modules", query={"id": self.module_ids["OPM"]})
            try:
                tenant.request("GET", "/admin/v1/me", expected=403, message="module not allowed")
                tenant.request("GET", "/admin/v1/perm-codes")
                for path, field in [("/admin/v1/routes", "items"), ("/admin/v1/initial-context", "menus")]:
                    menus = tenant.request("GET", path).get(field, [])
                    require(not any(node.get("component", "").startswith("app/opm/") for node in menu_nodes(menus)),
                            "removed OPM module remained in a nested menu")
                    require(all(node.get("children") for node in menu_nodes(menus) if node.get("component") == "BasicLayout"),
                            "empty catalog remained after module removal")
            finally:
                self.create_module("OPM")
            self.assert_profile(tenant, self.primary)
            restored = tenant.request("GET", "/admin/v1/routes").get("items", [])
            require(any(node.get("component", "").startswith("app/opm/") for node in menu_nodes(restored)),
                    "restored OPM module did not restore authorized menus")

        with step("移除 TENANT 白名单后租户管理菜单和租户用量接口均不可达"):
            tenant.request("GET", "/admin/v1/my-tenant-usage")
            self.admin.request("DELETE", "/admin/v1/plan-modules", query={"id": self.module_ids["TENANT"]})
            try:
                tenant.request("GET", "/admin/v1/my-tenant-usage", expected=403, message="module not allowed")
                for path, field in [("/admin/v1/routes", "items"), ("/admin/v1/initial-context", "menus")]:
                    menus = tenant.request("GET", path).get(field, [])
                    require(not any(node.get("component", "").startswith("app/tenant/") for node in menu_nodes(menus)),
                            "removed TENANT module remained in navigation")
            finally:
                self.create_module("TENANT")
            tenant.request("GET", "/admin/v1/my-tenant-usage")

        for status in ("OFF", "FREEZE", "EXPIRED"):
            with step("租户 " + status + " 时阻断登录和既有会话，启用后恢复"):
                self.admin.update("tenants", self.primary["id"], {"status": status})
                try:
                    tenant.request("GET", "/admin/v1/me", expected=403, message="not active")
                    self.new_client().login(self.username, self.initial_password, self.primary["code"], expected=403)
                finally:
                    self.admin.update("tenants", self.primary["id"], {"status": "ON"})
                self.assert_profile(tenant, self.primary)

        with step("套餐到期只读：允许查询、拒绝写入，续期恢复"):
            self.admin.update("tenants", self.primary["id"], {"expiredAt": timestamp(-1)})
            try:
                self.assert_profile(tenant, self.primary)
                tenant.request("POST", "/admin/v1/users", {"data": {"username": "expired_" + self.report.run_id[-6:]}},
                               expected=403, message="read-only")
            finally:
                self.admin.update("tenants", self.primary["id"], {"expiredAt": timestamp(30)})
            self.assert_profile(tenant, self.primary)
            self.assert_member_creation_restored(tenant, "renewed")

        with step("租户用量和四类配额与套餐关联一致"):
            usage = self.admin.request("GET", "/admin/v1/tenants/" + str(self.primary["id"]) + "/usage")
            require(usage.get("planId") == self.plan_id and int(usage["userCount"]) == 1, "tenant usage identity/count mismatch")
            require({item["quotaType"] for item in usage.get("quotas", [])} == set(self.quota_ids), "usage quota types mismatch")

        with step("错误旧密码不可改密，正确改密立即吊销旧会话"):
            tenant.request("POST", "/admin/v1/me/password", {
                "oldPassword": self.infra.encrypt(password()), "newPassword": self.infra.encrypt(self.final_password),
            }, expected=(400, 403))
            tenant.request("POST", "/admin/v1/me/password", {
                "oldPassword": self.infra.encrypt(self.initial_password), "newPassword": self.infra.encrypt(self.final_password),
            })
            self.current_password = self.final_password
            tenant.request("GET", "/admin/v1/me", expected=401)

        with step("旧密码登录失败，新密码登录成功"):
            self.new_client().login(self.username, self.initial_password, self.primary["code"], expected=400, reason="INVALID_PASSWORD")
            tenant = self.new_client()
            tenant.login(self.username, self.current_password, self.primary["code"])
            self.assert_profile(tenant, self.primary)
            unaffected = self.new_client()
            unaffected.login(self.username, self.other_password, other["code"])
            self.assert_profile(unaffected, other)
            other_client = unaffected

        with step("平台管理员重置租户密码，重置前会话失效"):
            reset_password = password()
            self.report.secrets.append(reset_password)
            self.admin.request("POST", "/admin/v1/users/" + str(self.primary["adminUserId"]) + "/password", {
                "newPassword": self.infra.encrypt(reset_password),
            })
            self.current_password = reset_password
            tenant.request("GET", "/admin/v1/me", expected=401)
            self.new_client().login(self.username, self.final_password, self.primary["code"],
                                    expected=400, reason="INVALID_PASSWORD")
            tenant = self.new_client()
            tenant.login(self.username, self.current_password, self.primary["code"])
            self.assert_profile(tenant, self.primary)
            unaffected = self.new_client()
            unaffected.login(self.username, self.other_password, other["code"])
            self.assert_profile(unaffected, other)
            other_client = unaffected

        with step("退出后访问令牌和刷新 Cookie 均失效"):
            tenant.request("POST", "/admin/v1/logout", {})
            tenant.request("GET", "/admin/v1/me", expected=401)
            tenant.request("POST", "/admin/v1/refresh-token", {"grant_type": "refresh_token"},
                           authenticated=False, expected=(400, 401))
            self.assert_profile(other_client, other)

        with step("临时租户数据清理、删除租户与存在性校验"):
            other_client.request("POST", "/admin/v1/logout", {})
            self.admin.request("POST", "/admin/v1/tenants/" + str(other["id"]) + "/cleanup", {})
            for resource in ("users", "roles"):
                rows = self.admin.request("GET", "/admin/v1/" + resource, query={
                    "query": json.dumps({"tenant_id": other["id"]}), "noPaging": "true",
                }).get("items", [])
                require(not rows, "temporary tenant still owns " + resource + " after cleanup")
            self.admin.request("DELETE", "/admin/v1/tenants/" + str(other["id"]))
            exists = self.admin.request("GET", "/admin/v1/tenants:exists", query={"code": other["code"]})
            require(not exists.get("exist"), "temporary tenant was not deleted")
            self.report.resources["tenants"][-1]["retained"] = False

        with step("临时套餐、模块、配额的更新与删除闭环"):
            scratch = self.create_plan("delete")
            scratch_id = scratch["id"]
            self.admin.request("POST", "/admin/v1/plan-modules", {"data": {"planId": scratch_id, "module": "TASK"}})
            module = self.admin.find("plan-modules", "module", "TASK", {"plan_id": scratch_id})
            self.admin.update("plan-modules", module["id"], {"module": "FILE"})
            require(self.admin.request("GET", "/admin/v1/plan-modules/" + str(module["id"]))["module"] == "FILE", "module update failed")
            self.admin.request("DELETE", "/admin/v1/plan-modules", query={"id": module["id"]})
            for kind in self.quota_ids:
                self.admin.request("POST", "/admin/v1/plan-quotas", {"data": {"planId": scratch_id, "quotaType": kind, "quotaValue": "1"}})
                quota = self.admin.find("plan-quotas", "quotaType", kind, {"plan_id": scratch_id})
                self.admin.update("plan-quotas", quota["id"], {"quotaValue": "2"})
                self.admin.request("DELETE", "/admin/v1/plan-quotas", query={"id": quota["id"]})
            remaining = self.admin.request("GET", "/admin/v1/plan-quotas", query={
                "query": json.dumps({"plan_id": scratch_id}), "noPaging": "true",
            }).get("items", [])
            require(not remaining, "deleted scratch quotas are still visible")
            self.admin.request("DELETE", "/admin/v1/plans", query={"id": scratch_id})
            rows = self.admin.request("GET", "/admin/v1/plans", query={"query": json.dumps({"name": scratch["name"]})})
            require(not any(row.get("id") == scratch_id for row in rows.get("items", [])), "temporary plan was not deleted")
            self.report.resources["plans"][-1]["retained"] = False

        with step("保留的新租户最终登录验收与退出"):
            final = self.new_client()
            final.login(self.username, self.current_password, self.primary["code"])
            self.assert_profile(final, self.primary)
            final.request("GET", "/admin/v1/initial-context")
            final.request("POST", "/admin/v1/logout", {})
            self.report.ready = True

    def credentials(self):
        return {
            "login_url": os.environ.get("E2E_MERCHANT_URL", "http://localhost:15889").rstrip("/") + "/auth/login?redirect=%2F", "ready": self.report.ready,
            "tenant_code": self.primary["code"] if self.primary else None,
            "tenant_id": self.primary["id"] if self.primary else None,
            "username": self.username, "password": self.current_password, "plan_id": self.plan_id,
        }


def main():
    report = Report()
    flow = Lifecycle(report)
    failure = None
    try:
        flow.run()
    except Exception as error:
        failure = report.redact(error)
        report.failure_trace = report.redact(traceback.format_exc())
        print("FAIL: " + failure, file=sys.stderr)
    finally:
        report.save(flow.base, flow.credentials(), failure)
    return 1 if failure else 0


if __name__ == "__main__":
    sys.exit(main())
