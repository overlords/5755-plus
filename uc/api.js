/* 用户中心 H5 — 数据层 + bridge 封装
 * 规格:docs/06a-user-center-h5-page.md §3/§6/§7;鉴权 ADR-0010。
 *
 * 数据面 /api/uc/v2/* 尚未实现(见 06a),故默认走 USE_MOCK。
 * 真接口就位后把 USE_MOCK 置 false 即可,fetch 路径已按 06a §3 写好。
 */

const UC = (() => {
  const USE_MOCK = true;                 // ← /api/uc/v2 就位后改 false
  const BASE = '/api/uc/v2';

  // ---- platformToken:加载即读入内存并抹除可见 URL(06a §7) ----
  let platformToken = '';
  (function captureToken() {
    const u = new URL(location.href);
    platformToken = u.searchParams.get('token') || '';
    if (u.searchParams.has('token')) {
      u.searchParams.delete('token');
      history.replaceState(null, '', u.pathname + u.search + u.hash);
    }
  })();

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
    if (res.status === 401) { handleInvalid(); throw new Error('session_invalid'); }
    const json = await res.json();
    // 约定:ApiResult { ok, data, reason, message }(04 口径)
    if (json && json.ok === false) {
      if (json.reason === 'platform_account_invalid') { handleInvalid(); }
      throw new Error(json.message || json.reason || 'request_failed');
    }
    return json.data;
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
