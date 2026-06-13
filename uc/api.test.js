/* uc SPA 数据层单测(node:test;不入部署)。规格:06a §3/§7。 */
const { test } = require('node:test');
const assert = require('node:assert');
const { captureToken, classifyResponse } = require('./api.js');

test('captureToken 从 ?token= 取出 token', () => {
  const { token } = captureToken('https://uc.xingninghuyu.com/?token=abc123');
  assert.strictEqual(token, 'abc123');
});

test('captureToken 抹除 URL 里的 token,保留其他 query/hash', () => {
  const { cleanUrl } = captureToken('https://uc.xingninghuyu.com/?token=abc123&foo=1#/orders');
  assert.ok(!cleanUrl.includes('token='), 'cleanUrl 不应含 token: ' + cleanUrl);
  assert.ok(cleanUrl.includes('foo=1'), '应保留其他 query: ' + cleanUrl);
  assert.ok(cleanUrl.includes('#/orders'), '应保留 hash: ' + cleanUrl);
});

test('captureToken 无 token:token 空、had=false、URL 不变(回归守卫)', () => {
  const r = captureToken('https://uc.xingninghuyu.com/?a=2#/home');
  assert.strictEqual(r.token, '');
  assert.strictEqual(r.had, false);
  assert.ok(r.cleanUrl.includes('a=2') && r.cleanUrl.includes('#/home'), r.cleanUrl);
});

test('classifyResponse 401 → session_invalid 失效', () => {
  const r = classifyResponse(401, null);
  assert.strictEqual(r.invalid, true);
});

test('classifyResponse ok=false + platform_account_invalid → 失效', () => {
  const r = classifyResponse(200, { ok: false, reason: 'platform_account_invalid', message: '账户失效' });
  assert.strictEqual(r.invalid, true);
});

test('classifyResponse ok=false + 其他 reason → 普通错误(非失效)', () => {
  const r = classifyResponse(200, { ok: false, reason: 'sms_code_wrong', message: '验证码错误' });
  assert.strictEqual(r.invalid, false);
  assert.strictEqual(r.error, '验证码错误');
});

test('classifyResponse 成功 → 非失效、无错误、返回 data', () => {
  const r = classifyResponse(200, { ok: true, data: { nickname: '云起玩家' } });
  assert.strictEqual(r.invalid, false);
  assert.strictEqual(r.error, null);
  assert.deepStrictEqual(r.data, { nickname: '云起玩家' });
});
