#!/usr/bin/env node
/**
 * OIDC SSO 全流冒烟 e2e（CI / 本地通用）。
 *
 * 环境变量：
 *   BASE_URL          后端地址（默认 http://localhost:7788）
 *   KC_BASE_URL       Keycloak 地址（默认 http://localhost:8081）
 *   KC_ADMIN / KC_ADMIN_PASSWORD  Keycloak 管理员凭据（默认 admin/admin）
 *   REDIS_HOST / REDIS_PORT / REDIS_PASSWORD  验证码答案读取（默认 127.0.0.1:6379 / *Abcd123456）
 *
 * 流程：接口同步 → SSO 开关 → 授权 URL（state）→ Keycloak 登录表单模拟
 * → 回调 code → SsoLogin 换 JWT → perm-codes 验证 → state 重放拒绝。
 *
 * 前置：backend 已以 SSO_OIDC_* 指向同一 Keycloak 启动；setup-keycloak.mjs 已执行。
 */
import { execFileSync } from 'node:child_process';
import crypto from 'node:crypto';

const BASE = process.env.BASE_URL || 'http://localhost:7788';
const KC = process.env.KC_BASE_URL || 'http://localhost:8081';
const REDIS_HOST = process.env.REDIS_HOST || '127.0.0.1';
const REDIS_PORT = process.env.REDIS_PORT || '6379';
const REDIS_PASSWORD = process.env.REDIS_PASSWORD || '*Abcd123456';

// 登录密码与后端约定的 AES-128-CBC 加密（key=iv=同 16 字节，见 platform/pkg/crypto）。
const cipher = crypto.createCipheriv('aes-128-cbc', Buffer.from('f51d66a73d8a0927'), Buffer.from('f51d66a73d8a0927'));
const ADMIN_PW_CIPHER = Buffer.concat([cipher.update('Abcd@1234', 'utf8'), cipher.final()]).toString('base64');

let jar = new Map();
const storeCookies = (res) => {
  for (const line of res.headers.getSetCookie?.() ?? []) {
    const [pair] = line.split(';');
    const eq = pair.indexOf('=');
    if (eq > 0) jar.set(pair.slice(0, eq), pair.slice(eq + 1));
  }
};
const cookieHeader = () => [...jar.entries()].map(([k, v]) => `${k}=${v}`).join('; ');

// 登录验证码：答案在 Redis（go-admin:captcha:<id>）。
// CI（ubuntu runner 自带 redis-tools）走直连；本地 Windows 无 redis-cli，
// 兜底 docker exec 进 redis 容器执行（REDIS_CONTAINER 可显式指定）。
function redisGet(key) {
  const args = ['-h', REDIS_HOST, '-p', REDIS_PORT, '-a', REDIS_PASSWORD, '--no-auth-warning', 'GET', key];
  try {
    return execFileSync('redis-cli', args, { encoding: 'utf8' }).trim();
  } catch (e) {
    if (e.code !== 'ENOENT') throw e;
  }
  const container = process.env.REDIS_CONTAINER || discoverRedisContainer();
  return execFileSync('docker', ['exec', container, 'redis-cli', ...args], { encoding: 'utf8' }).trim();
}

function discoverRedisContainer() {
  const out = execFileSync('docker', ['ps', '--format', '{{.ID}} {{.Image}}'], { encoding: 'utf8' });
  for (const line of out.split('\n')) {
    const [id, ...img] = line.trim().split(/\s+/);
    if (img.join(' ').startsWith('redis')) return id;
  }
  throw new Error('redis-cli not on PATH and no redis container found');
}

function captchaAnswer(id) {
  return redisGet(`go-admin:captcha:${id}`);
}

async function adminLogin() {
  const cap = await (await fetch(`${BASE}/admin/v1/captcha`)).json();
  const answer = captchaAnswer(cap.captchaId);
  if (!answer) throw new Error('empty captcha answer');
  const r = await fetch(`${BASE}/admin/v1/login`, {
    method: 'POST',
    headers: { 'content-type': 'application/json', 'X-Captcha-Id': cap.captchaId, 'X-Captcha-Value': answer },
    body: JSON.stringify({ username: 'admin', password: ADMIN_PW_CIPHER, grant_type: 'password', client_type: 'admin' }),
  });
  const body = await r.json();
  if (!body.access_token) throw new Error('admin login failed: ' + JSON.stringify(body).slice(0, 160));
  return body.access_token;
}

// ---- 主流程 ----
const adminToken = await adminLogin();
console.log('[e2e] platform admin login OK');

// 1. SSO 开关
const info = await (await fetch(`${BASE}/admin/v1/sso/login-info`)).json();
if (!info.enabled) throw new Error('SSO should be enabled');
console.log('[e2e] sso enabled OK');

// 2. 授权 URL + state
const urlRes = await (await fetch(`${BASE}/admin/v1/sso/login-url`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: '{}' })).json();
const authUrl = new URL(urlRes.authorizationUrl);
if (authUrl.searchParams.get('state') !== urlRes.state) throw new Error('state mismatch in auth url');
console.log('[e2e] auth url OK (client:', authUrl.searchParams.get('client_id') + ')');

// 3. Keycloak 登录表单模拟（真实 discovery/登录页/凭据 POST）
jar = new Map();
let resp = await fetch(urlRes.authorizationUrl, { redirect: 'manual' });
storeCookies(resp);
for (let i = 0; i < 5 && (resp.status === 302 || resp.status === 303); i++) {
  const loc = resp.headers.get('location');
  resp = await fetch(loc.startsWith('http') ? loc : KC + loc, { redirect: 'manual', headers: { cookie: cookieHeader() } });
  storeCookies(resp);
}
const loginHtml = await resp.text();
const action = loginHtml.match(/action="([^"]+)"/)?.[1].replace(/&amp;/g, '&');
if (!action) throw new Error('Keycloak login form action not found');

const form = new URLSearchParams({ username: 'sso-user', password: 'Keycloak@123', credentialId: '' });
for (const m of loginHtml.matchAll(/<input[^>]*name="([^"]+)"[^>]*value="([^"]*)"[^>]*>/g)) {
  if (m[1] !== 'username' && m[1] !== 'password' && m[1] !== 'credentialId') form.set(m[1], m[2]);
}
resp = await fetch(action, {
  method: 'POST', redirect: 'manual',
  headers: { 'content-type': 'application/x-www-form-urlencoded', cookie: cookieHeader() },
  body: form.toString(),
});
storeCookies(resp);
let code = null, state = null;
for (let i = 0; i < 8; i++) {
  if (resp.status === 302 || resp.status === 303) {
    const loc = resp.headers.get('location');
    if (loc.includes('code=')) {
      const u = new URL(loc);
      code = u.searchParams.get('code'); state = u.searchParams.get('state');
      break;
    }
    resp = await fetch(loc.startsWith('http') ? loc : KC + loc, { redirect: 'manual', headers: { cookie: cookieHeader() } });
    storeCookies(resp);
  } else break;
}
if (!code || state !== urlRes.state) throw new Error('callback code/state capture failed');
console.log('[e2e] callback captured (code len', code.length + ', state match)');

// 4. SsoLogin 换 JWT
const ssoRes = await fetch(`${BASE}/admin/v1/sso/login`, {
  method: 'POST', headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ code, state }),
});
const ssoBody = await ssoRes.json();
if (!ssoRes.ok || !ssoBody.access_token) throw new Error('SsoLogin failed: ' + JSON.stringify(ssoBody).slice(0, 200));
console.log('[e2e] SsoLogin OK, token len =', ssoBody.access_token.length);

// 5. JWT 可用
const meRes = await fetch(`${BASE}/admin/v1/perm-codes`, { headers: { authorization: 'Bearer ' + ssoBody.access_token } });
if (!meRes.ok) throw new Error('perm-codes with SSO JWT failed: ' + meRes.status);
console.log('[e2e] SSO JWT usable OK');

// 6. state 重放拒绝
const replay = await fetch(`${BASE}/admin/v1/sso/login`, {
  method: 'POST', headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ code, state }),
});
if (replay.ok) throw new Error('state replay should be rejected');
console.log('[e2e] state replay rejected OK');

console.log('SSO SMOKE ALL PASS');
