/* 用户中心 H5 — 路由 + 视图渲染
 * 版式:分组设置表(06a §1/§4);导航:页内 push + 子页返回(06a §2)。
 */

const $ = (sel, root = document) => root.querySelector(sel);
const app = $('#app');

// ---------- 通用 ----------
function toast(msg) {
  let t = $('.toast');
  if (!t) { t = document.createElement('div'); t.className = 'toast'; document.body.appendChild(t); }
  t.textContent = msg;
  requestAnimationFrame(() => t.classList.add('show'));
  clearTimeout(toast._t);
  toast._t = setTimeout(() => t.classList.remove('show'), 1600);
}

const esc = (s) => String(s == null ? '' : s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

// 验证码 60s 倒计时(口径同 07 §3)
function bindCountdown(btn, onSend) {
  btn.addEventListener('click', async () => {
    if (btn.disabled) return;
    try {
      await onSend();
    } catch (e) { toast('验证码发送失败'); return; }
    let n = 60;
    btn.disabled = true;
    btn.textContent = n + 's';
    const id = setInterval(() => {
      n -= 1;
      if (n <= 0) { clearInterval(id); btn.disabled = false; btn.textContent = '重新发送'; }
      else { btn.textContent = n + 's'; }
    }, 1000);
  });
}

function subhead(title) {
  return `<div class="subhead">
    <button class="back" aria-label="返回" onclick="history.back()">‹</button>
    <div class="title">${esc(title)}</div><div class="spacer"></div>
  </div>`;
}

// ---------- 主页 ----------
async function renderHome() {
  app.innerHTML = `<button class="uc-close" aria-label="关闭用户中心" onclick="UC.bridge && window.close && window.close()">×</button>
    <div class="view">
      ${UC.bridge.available() ? '' : '<div class="bridge-note">请在游戏内打开以使用切换小号 / 退出登录</div>'}
      <div class="card"><div class="identity">
        <div class="avatar skeleton"></div>
        <div class="identity-main"><div class="sk-line skeleton" style="width:60%"></div>
          <div class="sk-line skeleton" style="width:80%"></div></div>
      </div></div>
      <div class="card"><div class="sk-line skeleton" style="margin:20px"></div></div>
    </div>`;

  let p;
  try { p = await UC.getProfile(); }
  catch (e) {
    app.querySelector('.view').innerHTML = errorBlock(renderHome);
    return;
  }

  const bridgeOn = UC.bridge.available();
  const dis = bridgeOn ? '' : ' is-disabled';
  const avatar = p.avatarUrl
    ? `<img src="${esc(p.avatarUrl)}" alt="">`
    : esc((p.nickname || '玩')[0]);
  const realname = p.realNameStatus === 'verified'
    ? '<span class="badge ok">✓ 已实名</span>'
    : '<span class="badge off">未实名</span>';

  app.querySelector('.view').innerHTML = `
    ${bridgeOn ? '' : '<div class="bridge-note">请在游戏内打开以使用切换小号 / 退出登录</div>'}

    <div class="card"><div class="identity">
      <div class="avatar">${avatar}</div>
      <div class="identity-main">
        <div class="identity-name">${esc(p.nickname)}</div>
        <div class="identity-sub"><span class="identity-phone">${esc(p.maskedPhone)}</span>${realname}</div>
      </div>
    </div></div>

    <div class="group-title">当前小号</div>
    <div class="card">
      <button class="row${dis}" data-act="switch">
        <span class="row-label">${esc(p.currentSubAccount.label)}</span>
        <span class="row-value">切换</span><span class="row-chevron">›</span>
      </button>
    </div>

    <div class="group-title">账号安全</div>
    <div class="card">
      <button class="row" data-nav="#/phone"><span class="row-label">绑定手机</span>
        <span class="row-value">${esc(p.maskedPhone)}</span><span class="row-chevron">›</span></button>
      <button class="row" data-nav="#/password"><span class="row-label">修改密码</span>
        <span class="row-chevron">›</span></button>
      <div class="row" data-static><span class="row-label">实名认证</span>
        <span class="row-value">${p.realNameStatus === 'verified' ? '已实名' : '未实名'}</span></div>
    </div>

    <div class="group-title">我的</div>
    <div class="card">
      <button class="row" data-nav="#/orders"><span class="row-label">充值订单</span>
        <span class="row-chevron">›</span></button>
    </div>

    <button class="btn btn-secondary btn-logout${dis}" data-act="logout">退出登录</button>
  `;

  app.querySelectorAll('[data-nav]').forEach((el) =>
    el.addEventListener('click', () => { location.hash = el.dataset.nav; }));
  const sw = app.querySelector('[data-act="switch"]');
  if (sw && bridgeOn) sw.addEventListener('click', () => UC.bridge.switchAccount());
  const lo = app.querySelector('[data-act="logout"]');
  if (lo && bridgeOn) lo.addEventListener('click', () => {
    if (confirm('确认退出登录?SDK 会清理当前登录态。')) UC.bridge.logout();
  });
}

function errorBlock(retryFn) {
  setTimeout(() => {
    const b = app.querySelector('.retry-btn');
    if (b) b.addEventListener('click', retryFn);
  });
  return `<div class="error">加载失败<div class="retry"><button class="retry-btn">重试</button></div></div>`;
}

// ---------- 换绑手机 ----------
function renderPhone() {
  app.innerHTML = subhead('换绑手机') + `<div class="form">
    <div class="form-hint">输入新的手机号并完成短信验证后,绑定手机将更新。</div>
    <div class="field"><input class="input" id="newPhone" type="tel" maxlength="11" placeholder="请输入新手机号"></div>
    <div class="field"><div class="input-row">
      <input class="input" id="phoneCode" placeholder="请输入验证码">
      <button class="sms-btn" id="phoneSms">发送验证码</button>
    </div></div>
    <button class="btn btn-primary" id="phoneSubmit">确认换绑</button>
  </div>`;

  const phone = $('#newPhone');
  bindCountdown($('#phoneSms'), async () => {
    if (!/^1\d{10}$/.test(phone.value)) { toast('请输入正确的 11 位手机号'); throw new Error('bad'); }
    await UC.sendPhoneSms(phone.value);
    toast('验证码已发送');
  });
  $('#phoneSubmit').addEventListener('click', async () => {
    if (!/^1\d{10}$/.test(phone.value)) return toast('请输入正确的 11 位手机号');
    const code = $('#phoneCode').value.trim();
    if (!code) return toast('请输入验证码');
    try { await UC.rebindPhone(phone.value, code); }
    catch (e) { return toast(e.message === 'session_invalid' ? '登录已失效' : '换绑失败'); }
    sessionStorage.setItem('uc_flash', '换绑成功');
    history.back();
  });
}

// ---------- 修改密码 ----------
function renderPassword() {
  app.innerHTML = subhead('修改密码') + `<div class="form">
    <div class="form-hint">通过绑定手机的短信验证身份后设置新密码。修改成功需重新登录。</div>
    <div class="field"><div class="input-row">
      <input class="input" id="pwCode" placeholder="请输入验证码">
      <button class="sms-btn" id="pwSms">发送验证码</button>
    </div></div>
    <div class="field"><input class="input" id="pwNew" type="password" placeholder="请输入新密码"></div>
    <button class="btn btn-primary" id="pwSubmit">确认修改</button>
  </div>`;

  bindCountdown($('#pwSms'), async () => { await UC.sendPasswordSms(); toast('验证码已发送'); });
  $('#pwSubmit').addEventListener('click', async () => {
    const code = $('#pwCode').value.trim();
    const pw = $('#pwNew').value;
    if (!code) return toast('请输入验证码');
    if (!pw || pw.length < 6) return toast('新密码至少 6 位');
    try { await UC.changePassword(code, pw); }
    catch (e) { return toast(e.message === 'session_invalid' ? '登录已失效' : '修改失败'); }
    toast('密码已修改');
    // 改密 → platformToken 作废 → 强制重登(06a §3)
    setTimeout(() => UC.bridge.sessionInvalid(), 800);
  });
}

// ---------- 充值订单 ----------
async function renderOrders() {
  app.innerHTML = subhead('充值订单') + `<div id="orderList" class="view" style="padding-top:4px">
    <div class="card"><div class="order"><div class="sk-line skeleton" style="width:70%"></div>
      <div class="sk-line skeleton" style="width:40%"></div></div></div></div>`;
  const list = $('#orderList');
  let data;
  try { data = await UC.getOrders(); }
  catch (e) { list.innerHTML = errorBlock(renderOrders); return; }

  if (!data.orders.length) { list.innerHTML = '<div class="empty">暂无充值订单</div>'; return; }

  list.innerHTML = '<div class="card">' + data.orders.map((o) => {
    const amt = '¥' + Number(o.amount).toFixed(2);
    const st = o.status === 'done'
      ? '<span class="order-status done">已发放</span>'
      : '<span class="order-status pending">处理中</span>';
    return `<div class="order">
      <div class="order-top"><span class="order-name">${esc(o.productName)}</span><span class="order-amount">${amt}</span></div>
      <div class="order-bottom"><span class="order-meta">${esc(o.createdAt)} · ${esc(o.orderId)}</span>${st}</div>
    </div>`;
  }).join('') + '</div>';
}

// ---------- 路由 ----------
const routes = { '': renderHome, '#/': renderHome, '#/phone': renderPhone, '#/password': renderPassword, '#/orders': renderOrders };

function route() {
  const fn = routes[location.hash] || renderHome;
  window.scrollTo(0, 0);
  fn();
  const flash = sessionStorage.getItem('uc_flash');
  if (flash && (location.hash === '' || location.hash === '#/')) { sessionStorage.removeItem('uc_flash'); toast(flash); }
}

window.addEventListener('hashchange', route);
route();
