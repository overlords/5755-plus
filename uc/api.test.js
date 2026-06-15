/* uc SPA 数据层单测(node:test;不入部署)。规格:06a §3/§7。 */
const { test } = require('node:test');
const assert = require('node:assert');
const { captureToken, classifyResponse } = require('./api.js');

test('captureToken 从 #token= 取出 token(fragment,ADR-0018)', () => {
  const { token } = captureToken('https://uc.xingninghuyu.com/#token=abc123');
  assert.strictEqual(token, 'abc123');
});

test('captureToken 抹除 fragment 里的 token,保留其他 query、清空 hash', () => {
  const { cleanUrl } = captureToken('https://uc.xingninghuyu.com/?foo=1#token=abc123');
  assert.ok(!cleanUrl.includes('token='), 'cleanUrl 不应含 token: ' + cleanUrl);
  assert.ok(cleanUrl.includes('foo=1'), '应保留其他 query: ' + cleanUrl);
  assert.ok(!cleanUrl.includes('#token'), 'token 段应抹除: ' + cleanUrl);
});

test('captureToken 无 token:token 空、had=false、hash 路由原样保留(回归守卫)', () => {
  const r = captureToken('https://uc.xingninghuyu.com/?a=2#/orders');
  assert.strictEqual(r.token, '');
  assert.strictEqual(r.had, false);
  assert.ok(r.cleanUrl.includes('a=2') && r.cleanUrl.includes('#/orders'), r.cleanUrl);
});

test('classifyResponse 401 → session_invalid 失效', () => {
  const r = classifyResponse(401, null);
  assert.strictEqual(r.invalid, true);
});

test('classifyResponse ok=false + platform_account_invalid → 失效', () => {
  const r = classifyResponse(200, { success: false, reason: 'platform_account_invalid', message: '账户失效' });
  assert.strictEqual(r.invalid, true);
});

test('classifyResponse ok=false + 其他 reason → 普通错误(非失效)', () => {
  const r = classifyResponse(200, { success: false, reason: 'sms_code_wrong', message: '验证码错误' });
  assert.strictEqual(r.invalid, false);
  assert.strictEqual(r.error, '验证码错误');
});

test('classifyResponse 成功 → 非失效、无错误、返回 data', () => {
  const r = classifyResponse(200, { success: true, data: { nickname: '云起玩家' } });
  assert.strictEqual(r.invalid, false);
  assert.strictEqual(r.error, null);
  assert.deepStrictEqual(r.data, { nickname: '云起玩家' });
});

test('classifyResponse 真实后端失败响应 success:false → 有 error(防字段名写回 ok,致 4xx 误报成功)', () => {
  // 后端 ApiResult 字段是 success(result.go),不是 ok;曾因 classifyResponse 误查 json.ok,
  // 导致所有 4xx 业务错误(验证码错/密码不合规等)被漏判成「成功」、前端谎报「密码已修改」。
  const r = classifyResponse(400, { success: false, code: 3, reason: 'param_invalid', message: '验证码错误' });
  assert.strictEqual(r.invalid, false);
  assert.strictEqual(r.error, '验证码错误');
});
