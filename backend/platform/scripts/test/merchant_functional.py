"""Exercise disposable business fixtures in an existing local test merchant.

Use the retained tenant-permissions credentials, never an existing business tenant.
All business mutations use HTTP APIs; fixtures are removed in finally blocks.
"""

import base64
import hashlib
import hmac
import json
import os
from pathlib import Path
import sys
import struct
import time
import traceback

from tenant_lifecycle import Client, Infrastructure, Report, ROOT, password, require


def run(report, credentials):
    infra = Infrastructure(report)
    owner = Client(os.environ.get("E2E_BASE_URL", "http://localhost:15888"), report, infra)
    report.secrets.append(credentials["password"])
    failures = []
    suffix = report.run_id
    prefix = "/admin/v1/"

    def check(name, action):
        try:
            with report.step(name):
                action()
        except Exception as error:
            failures.append(name + ": " + str(error))
            print(report.redact(traceback.format_exc()), flush=True)

    with report.step("商户入口登录并核对租户身份"):
        owner.login(credentials["username"], credentials["password"], credentials["tenant_code"])
        me = owner.request("GET", prefix + "me")
        require(me.get("tenantId") == credentials["tenant_id"], "wrong merchant identity")

    with report.step("平台与商户入口隔离及伪造商户请求头校验"):
        entry_mode = os.environ.get("E2E_TENANT_ENTRY_MODE", "proxy")
        if entry_mode == "proxy":
            pinned = owner.request("GET", prefix + "me", headers={"X-Tenant-Code": "untrusted_merchant"})
            require(pinned.get("tenantId") == credentials["tenant_id"], "merchant proxy trusted a browser-selected merchant")
        elif entry_mode == "gateway":
            # A direct gateway resolves this header rather than overwriting it;
            # a forged merchant must therefore be rejected before dispatch.
            owner.request("GET", prefix + "me", headers={"X-Tenant-Code": "untrusted_merchant"}, expected=403)
        else:
            raise ValueError("E2E_TENANT_ENTRY_MODE must be proxy or gateway")
        platform_entry = Client(owner.base, report, infra)
        platform_entry.token = owner.token
        platform_entry.request("GET", prefix + "me", expected=403)

    def crud(resource, data, field="name", changes=None):
        owner.request("POST", prefix + resource, {"data": data})
        row = owner.find(resource, field, data[field])
        identifier = row["id"]
        report.resources.setdefault(resource, []).append(identifier)
        try:
            detail = owner.request("GET", prefix + resource + "/" + str(identifier))
            require(detail.get("tenantId") == credentials["tenant_id"], resource + " ownership mismatch")
            changes = changes or {"remark": "edited " + suffix}
            owner.update(resource, identifier, changes)
            detail = owner.request("GET", prefix + resource + "/" + str(identifier))
            require(all(detail.get(key) == value for key, value in changes.items()), resource + " update mismatch")
        finally:
            owner.request("DELETE", prefix + resource + "/" + str(identifier))
        owner.request("GET", prefix + resource + "/" + str(identifier), expected=404)

    check("组织新增、精确回查、编辑和删除", lambda: crud("org-units", {
        "name": "验收部门_" + suffix, "code": "qa_org_" + suffix,
        "type": "DEPARTMENT", "status": "ON",
    }))
    check("岗位新增、编辑、禁用和删除", lambda: crud("positions", {
        "name": "验收岗位_" + suffix, "code": "qa_pos_" + suffix,
        "type": "REGULAR", "status": "ON",
    }, changes={"description": "edited " + suffix, "status": "OFF"}))
    check("租户角色新增、编辑和删除", lambda: crud("roles", {
        "name": "验收临时角色_" + suffix, "code": "tenant:qa:" + suffix,
        "type": "TENANT", "status": "ON", "tenantId": credentials["tenant_id"],
        "permissions": [item["id"] for item in owner.request("GET", prefix + "permissions").get("items", [])],
    }, changes={"description": "edited " + suffix}))

    def users():
        name = "qa_" + suffix
        secret = password()
        report.secrets.append(secret)
        owner.request("POST", prefix + "users", {"data": {
            "username": name, "nickname": "新增测试成员", "status": "NORMAL",
            "roleIds": [credentials["reader"]["role_id"]],
        }, "password": secret})
        row = owner.find("users", "username", name)
        try:
            owner.update("users", row["id"], {"nickname": "编辑后成员"})
            require(owner.request("GET", prefix + "users/" + str(row["id"])).get("nickname") == "编辑后成员", "user edit mismatch")
            member = Client(owner.base, report, infra)
            member.login(name, secret, credentials["tenant_code"])
            member.request("GET", prefix + "users")
            member.request("POST", prefix + "users", {"data": {}}, expected=403)
            member.request("POST", prefix + "logout", {})
        finally:
            owner.request("DELETE", prefix + "users/" + str(row["id"]))
        owner.request("GET", prefix + "users/" + str(row["id"]), expected=404)

    check("成员新增、编辑、登录、只读权限和删除", users)

    def profile():
        old = owner.request("GET", prefix + "me")
        avatar = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jC1sAAAAASUVORK5CYII="
        try:
            owner.request("PUT", prefix + "me", {"data": {"nickname": "个人资料验收_" + suffix}, "updateMask": "nickname"})
            require(owner.request("GET", prefix + "me").get("nickname") == "个人资料验收_" + suffix, "profile update failed")
            result = owner.request("POST", prefix + "me/avatar", {"imageUrl": avatar})
            require(result.get("url") == avatar and owner.request("GET", prefix + "me").get("avatar") == avatar, "avatar update failed")
            owner.request("DELETE", prefix + "me/avatar")
            require(not owner.request("GET", prefix + "me").get("avatar"), "avatar deletion failed")
        finally:
            owner.request("PUT", prefix + "me", {"data": {"nickname": old.get("nickname", ""), "avatar": old.get("avatar", "")}, "updateMask": "nickname,avatar"})

    check("个人资料修改、头像设置和移除并恢复", profile)

    def mfa():
        name, secret = "mfa_" + suffix, password()
        report.secrets.append(secret)
        roles = owner.request("GET", prefix + "users/" + str(me["id"])).get("roleIds", [])
        require(bool(roles), "owner roles missing")
        owner.request("POST", prefix + "users", {"data": {"username": name, "status": "NORMAL", "roleIds": roles}, "password": secret})
        row = owner.find("users", "username", name)
        member = Client(owner.base, report, infra)
        try:
            member.login(name, secret, credentials["tenant_code"])
            enrollment = member.request("POST", prefix + "mfa/enroll/start", {"method": "TOTP"})
            totp_secret = enrollment["totp"]["secret"]
            report.secrets.append(totp_secret)
            def code():
                key = base64.b32decode(totp_secret + "=" * ((-len(totp_secret)) % 8))
                digest = hmac.new(key, struct.pack(">Q", int(time.time()) // 30), hashlib.sha1).digest()
                offset = digest[-1] & 15
                return str((struct.unpack(">I", digest[offset:offset + 4])[0] & 0x7fffffff) % 1000000).zfill(6)
            result = member.request("POST", prefix + "mfa/enroll/confirm", {"method": "TOTP", "operationId": enrollment["operationId"], "totpCode": code()})
            require(result.get("success"), "TOTP enrollment failed")
            require(member.request("GET", prefix + "mfa/status").get("enabled"), "MFA was not enabled")
            challenge_client = Client(owner.base, report, infra)
            challenge_client.tenant_code = credentials["tenant_code"]
            challenge = challenge_client.request("POST", prefix + "login", {"grant_type": "password", "username": name,
                "password": infra.encrypt(secret)}, authenticated=False, headers=infra.captcha(challenge_client))
            require(not challenge.get("access_token") and challenge.get("mfa_operation_id"), "password bypassed MFA")
            result = challenge_client.request("POST", prefix + "mfa/verify", {"operationId": challenge["mfa_operation_id"], "totpCode": code()}, authenticated=False)
            require(bool(result.get("access_token")), "MFA challenge did not issue access token")
            report.secrets.append(result["access_token"])
            challenge_client.token = result["access_token"]
            require(challenge_client.request("GET", prefix + "me").get("id") == row["id"], "MFA issued wrong identity")
            member.request("POST", prefix + "mfa/disable", {"method": "TOTP", "totpCode": code()})
            require(not member.request("GET", prefix + "mfa/status").get("enabled"), "MFA disable failed")
        finally:
            owner.request("DELETE", prefix + "users/" + str(row["id"]))

    check("临时成员TOTP绑定、二次登录验证和解绑", mfa)

    def sessions():
        other = Client(owner.base, report, infra)
        other.login(credentials["username"], credentials["password"], credentials["tenant_code"])
        rows = other.request("GET", prefix + "online-session/my-sessions").get("items", [])
        current = next(item for item in rows if item.get("current"))
        require(current.get("userAgent") == "GoAdminMerchantQA/1.0", "gRPC client identity replaced the HTTP device identity")
        owner.request("POST", prefix + "online-session/my-sessions/revoke", {"jti": current["jti"], "clientType": current["clientType"]})
        other.request("GET", prefix + "me", expected=401)
        owner.request("GET", prefix + "me")

    check("查看个人会话、撤销指定测试会话并验证立即失效", sessions)

    def tasks():
        names = owner.request("GET", prefix + "tasks:type-names").get("typeNames", [])
        require(bool(names), "no registered task type")
        owner.request("GET", prefix + "tasks")
        owner.request("POST", prefix + "tasks", {"data": {"type": "PERIODIC", "typeName": names[0],
                       "cronSpec": "0 0 * * *", "taskPayload": "{}", "enable": False}}, expected=403)
        owner.request("POST", prefix + "tasks:control", {"typeName": names[0], "controlType": "Stop"}, expected=403)

    check("任务列表与类型读取、平台调度写入隔离", tasks)

    def preferences():
        old = owner.request("GET", prefix + "notification-preference")
        keys = ("quietEnabled", "quietStartMinute", "quietEndMinute", "mutedCategoryIds")
        restore = {key: old.get(key) for key in keys if key in old}
        try:
            owner.request("POST", prefix + "notification-preference", {
                "quietEnabled": True, "quietStartMinute": 0, "quietEndMinute": 60,
                "mutedCategoryIds": [],
            })
            saved = owner.request("GET", prefix + "notification-preference")
            require(saved.get("quietStartMinute") == 0 and saved.get("quietEndMinute") == 60, "midnight preference lost")
            owner.request("POST", prefix + "notification-preference", {
                "quietEnabled": True, "quietStartMinute": 60, "quietEndMinute": 0,
                "mutedCategoryIds": [],
            })
            require(owner.request("GET", prefix + "notification-preference").get("quietEndMinute") == 0, "midnight end lost")
        finally:
            owner.request("POST", prefix + "notification-preference", restore)

    check("通知偏好保存零点、跨日并恢复", preferences)

    def messages():
        # Send only to our own test owner. No email/SMS/third-party recipient.
        result = owner.request("POST", prefix + "internal-message/send", {
            "type": "NOTIFICATION", "targetUserIds": [me["id"]], "targetAll": False,
            "title": "验收通知_" + suffix, "content": "商户内部通知功能验收",
        })
        message_id = result["messageId"]
        try:
            owner.request("GET", prefix + "internal-message/messages/" + str(message_id))
            inbox = owner.request("GET", prefix + "internal-message/inbox", query={"noPaging": "true"}).get("items", [])
            row = next(item for item in inbox if item.get("messageId") == message_id)
            owner.request("POST", prefix + "internal-message/read", {"userId": me["id"], "recipientIds": [row["id"]]})
            inbox = owner.request("GET", prefix + "internal-message/inbox", query={"noPaging": "true"}).get("items", [])
            require(next(item for item in inbox if item.get("messageId") == message_id).get("status") == "READ", "read status mismatch")
        finally:
            owner.request("POST", prefix + "internal-message/revoke", {"messageId": message_id, "userId": 0})
        require(not any(item.get("messageId") == message_id for item in owner.request("GET", prefix + "internal-message/inbox").get("items", [])), "revoked message remained")

    check("站内通知发送、收件、已读和撤回", messages)

    def files():
        content = ("merchant functional test " + suffix + "\n").encode()
        name = "qa_" + suffix + ".txt"
        owner.request("PUT", prefix + "file/upload", {"storageObject": {"fileDirectory": "merchant-qa/" + suffix},
            "file": base64.b64encode(content).decode(), "sourceFileName": name,
            "mime": "text/plain", "size": len(content)})
        row = owner.find("files", "fileName", name)
        try:
            data = owner.request("GET", prefix + "file/download", query={"fileId": row["id"], "preferPresignedUrl": "false"}, binary=True)
            require(data == content, "file download differs from upload")
            data = owner.request("GET", prefix + "file/download", query={"fileId": row["id"], "rangeStart": 0, "rangeEnd": 8, "preferPresignedUrl": "false"}, expected=(200, 206), binary=True)
            require(data == content[:8], "range download mismatch")
        finally:
            owner.request("DELETE", prefix + "files/" + str(row["id"]))
        owner.request("GET", prefix + "files/" + str(row["id"]), expected=404)

    check("文件真实上传、下载、范围读取和删除", files)

    def access_keys():
        created = owner.request("POST", prefix + "access-keys", {"data": {"name": "qa_" + suffix, "status": "ON"}})
        row, secret = created["data"], created["secret"]
        report.secrets.append(secret)
        try:
            require(row.get("tenantId") == credentials["tenant_id"], "AK ownership mismatch")
            def exchange(key, expected=200):
                result = owner.request("POST", prefix + "access-keys/token", {"accessKey": row["accessKey"], "secret": key}, expected=expected, authenticated=False)
                if result.get("accessToken"):
                    report.secrets.append(result["accessToken"])
                return result
            require(bool(exchange(secret).get("accessToken")), "AK exchange returned no token")
            owner.update("access-keys", row["id"], {"name": "edited " + suffix})
            rotated = owner.request("PUT", prefix + "access-keys/" + str(row["id"]) + "/secret", {})["secret"]
            report.secrets.append(rotated)
            exchange(secret, expected=400)
            exchange(rotated)
            owner.update("access-keys", row["id"], {"status": "OFF"})
            exchange(rotated, expected=400)
        finally:
            owner.request("DELETE", prefix + "access-keys/" + str(row["id"]))

    check("访问凭证创建、换令牌、轮换、禁用和删除", access_keys)

    def agents():
        graph = {"nodes": [
            {"id": "start_1", "type": "start", "position": {"x": 0, "y": 0}, "data": {"inputVars": [{"name": "question", "required": True}]}},
            {"id": "end_1", "type": "end", "position": {"x": 300, "y": 0}, "data": {"output": "echo:{{start_1.question}}"}},
        ], "edges": [{"id": "e1", "source": "start_1", "target": "end_1"}]}
        row = owner.request("POST", prefix + "ai/agents", {"data": {"name": "qa_" + suffix,
            "graph": json.dumps(graph), "enabled": True, "tenantId": credentials["tenant_id"]}})
        try:
            owner.update("ai/agents", row["id"], {"description": "edited " + suffix})
            require(owner.request("GET", prefix + "ai/agents/" + str(row["id"])).get("description") == "edited " + suffix, "agent edit mismatch")
            result = owner.request("POST", prefix + "ai/agents/" + str(row["id"]) + "/run", {"input": {"question": "smoke"}})
            require(result.get("status") == "SUCCEEDED" and result.get("output") == "echo:smoke", "agent run failed: " + str(result))
            require(owner.request("GET", prefix + "ai/agent-runs/" + str(result["id"])).get("output") == "echo:smoke", "run ledger mismatch")
            owner.request("GET", prefix + "ai/agent-runs", query={"query": json.dumps({"appId": row["id"]})})
        finally:
            owner.request("DELETE", prefix + "ai/agents/" + str(row["id"]))

    check("Agent新增、编辑、真实运行、运行台账和删除", agents)

    def role_boundaries():
        admin = Client(owner.base, report, infra)
        admin_password = os.environ.get("ADMIN_SMOKE_PASSWORD", "Abcd@1234")
        report.secrets.append(admin_password)
        admin.login(os.environ.get("ADMIN_SMOKE_USERNAME", "admin"), admin_password)
        try:
            merchant_entry = Client(owner.base, report, infra)
            merchant_entry.tenant_code = credentials["tenant_code"]
            merchant_entry.token = admin.token
            merchant_entry.request("GET", prefix + "me", expected=403)
            permission_id = admin.find("permissions", "code", "sys:platform_admin")["id"]
        finally:
            admin.request("POST", prefix + "logout", {})
        code = "tenant:qa_boundary:" + suffix
        try:
            owner.request("POST", prefix + "roles", {"data": {"name": code, "code": code,
                "type": "TENANT", "status": "ON", "permissions": [permission_id]}}, expected=403)
            owner.request("POST", prefix + "roles", {"data": {"name": code, "code": code, "type": "TENANT", "status": "ON"}})
            row = owner.find("roles", "code", code)
            owner.request("PUT", prefix + "roles/" + str(row["id"]), {
                "data": {"permissions": [permission_id]}, "updateMask": "permissions"}, expected=403)
            require(not owner.request("GET", prefix + "roles/" + str(row["id"])).get("permissions"), "rejected grant still modified role")
        finally:
            rows = owner.request("GET", prefix + "roles", query={"query": json.dumps({"code": code}), "noPaging": "true"}).get("items", [])
            for row in rows:
                if row.get("code") == code:
                    owner.request("DELETE", prefix + "roles/" + str(row["id"]))

    check("商户角色新增与编辑均拒绝授予平台权限", role_boundaries)

    def boundaries():
        for path in ["tasks:start", "tasks:stop", "tasks:restart", "system-tasks:inspect", "dashboard/ai-insights", "apis/sync"]:
            owner.request("POST", prefix + path, {}, expected=403)
        owner.request("POST", prefix + "ai/providers", {"data": {}}, expected=403)
        owner.request("GET", prefix + "online-session/sessions", expected=403)

    check("商户不能操作全平台任务、会话、接口与模型配置", boundaries)
    with report.step("记录外部集成未配置的实际限制"):
        providers = owner.request("GET", prefix + "ai/providers").get("items", [])
        report.resources["external_integrations"] = [{
            "status": "NOT_TESTED", "feature": "LLM聊天/知识库向量检索/AI内容生成",
            "reason": "当前没有模型供应商；PostgreSQL未安装pgvector" if not providers else "需要明确的外部模型与计费配置",
        }, {"status": "NOT_TESTED", "feature": "外部邮件/短信/第三方通知", "reason": "本次只发送测试租户内部通知"}]
    owner.request("POST", prefix + "logout", {})
    require(not failures, "; ".join(failures))


def main():
    path = Path(os.environ.get("E2E_CREDENTIALS", ROOT / "platform/artifacts/tenant-permissions/latest.json"))
    document = json.loads(path.read_text())
    credentials = json.loads(Path(document["credentials"]).read_text()) if "credentials" in document else document
    require(credentials.get("ready") and credentials.get("reader"), "use a ready local tenant-permissions fixture")
    require(credentials.get("tenant_code", "").startswith("e2e_"), "only disposable E2E merchants may run business mutation tests")
    os.environ.setdefault("E2E_TENANT_BASE_URL", "http://localhost:15889")
    report = Report(ROOT / "platform/artifacts/merchant-functional", title="商户功能自动化验收")
    failure = None
    try:
        run(report, credentials)
    except Exception as error:
        failure = error
        report.failure_trace = report.redact(traceback.format_exc())
        print(report.failure_trace, file=sys.stderr)
    report.save(os.environ["E2E_TENANT_BASE_URL"], credentials, failure)
    return 1 if failure else 0


if __name__ == "__main__":
    raise SystemExit(main())
