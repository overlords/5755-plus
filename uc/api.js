/* 用户中心 H5 — 数据层 + bridge 封装
 * 规格:docs/06a-user-center-h5-page.md §3/§6/§7;鉴权 ADR-0010。
 *
 * 数据面 /api/uc/v2/* 六端点服务端已实现(internal/api/api_uc.go),real 已逐一对齐。
 * 仍默认 USE_MOCK=true;翻 false 前还需两件部署侧事:
 *   1) 网络路径:BASE 相对路径会打到 SPA 自身域(uc.*),需 nginx 反代 /api/uc/v2 →
 *      平台服务端,或把 BASE 改绝对域(CORS 已默认放行 uc.xingninghuyu.com);
 *   2) 把含这批端点的服务端部署到 dev(CTID 105)。
 */

// ---- platformToken 捕获(纯函数,06a §7;可测)----
// 取 ?token= 值并产出抹除 token 的 URL(保留其他 query/hash)。
function captureToken(href) {
  const u = new URL(href);
  const token = u.searchParams.get('token') || '';
  const had = u.searchParams.has('token');
  if (had) u.searchParams.delete('token');
  return { token, had, cleanUrl: u.pathname + u.search + u.hash };
}

// ---- 响应失效收口(纯函数,06a §3;可测)----
// 决定一次响应是:失效(invalid,→ 上报 session_invalid)/ 普通错误(error)/ 成功(data)。
function classifyResponse(status, json) {
  if (status === 401) {
    return { invalid: true, error: 'session_invalid', data: null };
  }
  if (json && json.ok === false) {
    return {
      invalid: json.reason === 'platform_account_invalid',
      error: json.message || json.reason || 'request_failed',
      data: null,
    };
  }
  return { invalid: false, error: null, data: json ? json.data : undefined };
}

const UC = (() => {
  const USE_MOCK = true;                 // ← /api/uc/v2 就位后改 false
  const BASE = '/api/uc/v2';

  // ---- platformToken:加载即读入内存并抹除可见 URL(06a §7) ----
  let platformToken = '';
  if (typeof location !== 'undefined') {
    const cap = captureToken(location.href);
    platformToken = cap.token;
    if (cap.had) {
      history.replaceState(null, '', cap.cleanUrl);
    }
  }

  // ---- bridge 封装(06 §3 / 06a §6) ----
  const bridge = {
    available() { return !!(window.UserCenter && typeof window.UserCenter.postAccountAction === 'function'); },
    post(action) {
      if (this.available()) { window.UserCenter.postAccountAction(action); }
      else { console.warn('[uc] bridge 不可用,忽略动作:', action); }
    },
    switchAccount() { this.post('switch_account'); },
    logout() { this.post('logout'); },
    sessionInvalid() { this.post('session_invalid'); },
  };

  // ---- 失效收口:任一调用得到 platform_account_invalid/401 → session_invalid ----
  function handleInvalid() { bridge.sessionInvalid(); }

  async function call(method, path, body) {
    const res = await fetch(BASE + path, {
      method,
      headers: {
        'Content-Type': 'application/json',
        'X-M5755-Platform-Token': platformToken,
      },
      body: body ? JSON.stringify(body) : undefined,
    });
    // 约定:ApiResult { ok, data, reason, message }(04 口径);失效收口走 classifyResponse(已单测,06a §3)
    const json = res.status === 401 ? null : await res.json();
    const r = classifyResponse(res.status, json);
    if (r.invalid) { handleInvalid(); }
    if (r.error) { throw new Error(r.error); }
    return r.data;
  }

  // ---- 真实 API(06a §3) ----
  const real = {
    getProfile: () => call('GET', '/profile'),
    getOrders: (cursor) => call('GET', '/orders' + (cursor ? '?cursor=' + encodeURIComponent(cursor) : '')),
    sendPhoneSms: (newPhone) => call('POST', '/phone/sms-codes', { newPhone }),
    rebindPhone: (newPhone, smsCode) => call('PUT', '/phone', { newPhone, smsCode }),
    sendPasswordSms: () => call('POST', '/password/sms-codes'),
    changePassword: (smsCode, newPassword) => call('PUT', '/password', { smsCode, newPassword }),
  };

  // ---- mock(/api/uc/v2 未就位时;字段口径同 real) ----
  const wait = (ms) => new Promise((r) => setTimeout(r, ms));
  let mockPhone = '138****6677';
  const mock = {
    async getProfile() {
      await wait(450);
      return {
        nickname: '云起玩家',
        maskedPhone: mockPhone,
        avatarUrl: null,
        realNameStatus: 'verified',                 // verified | unverified
        currentSubAccount: { account: 'sub_1', label: '云起·一区' },
      };
    },
    async getOrders(cursor) {
      await wait(400);
      if (cursor) return { orders: [], nextCursor: null };
      return {
        orders: [
          { orderId: 'UO2026061300137', productName: '6480 元宝', amount: 648.0, currency: 'CNY', createdAt: '2026-06-12 21:14', status: 'done' },
          { orderId: 'UO2026061100092', productName: '1280 元宝', amount: 128.0, currency: 'CNY', createdAt: '2026-06-11 12:03', status: 'done' },
          { orderId: 'UO2026061000031', productName: '30 日卡', amount: 30.0, currency: 'CNY', createdAt: '2026-06-10 09:41', status: 'pending' },
        ],
        nextCursor: null,
      };
    },
    async sendPhoneSms() { await wait(300); return { sent: true }; },
    async rebindPhone(newPhone) { await wait(400); mockPhone = newPhone.slice(0, 3) + '****' + newPhone.slice(-4); return { ok: true }; },
    async sendPasswordSms() { await wait(300); return { sent: true }; },
    async changePassword() { await wait(400); return { ok: true }; },
  };

  return { bridge, ...(USE_MOCK ? mock : real), USE_MOCK };
})();

// 双模导出:浏览器用 const UC 全局;Node(测试)可 require 纯函数。
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { captureToken, classifyResponse };
}
