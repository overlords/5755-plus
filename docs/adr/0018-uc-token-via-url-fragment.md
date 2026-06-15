# 用户中心 `platformToken` 经 URL fragment(`#token=`)传入 WebView,而非 query / 请求头 / cookie 会话

状态:已接受(2026-06-16)

## 背景

用户中心是平台远程 SPA(`userCenterUrl`,06a),被 SDK 的 WebView 抽屉加载,**以主账户为核心**。SPA 自己拿着 `platformToken` 发 `/api/uc/v2/*` 的 Bearer 数据请求(06a §3),所以 token **必须送进页面 JS**。

原口径(04 §2.1 / 06 §5 / 06a §7 旧文 / 07)是「`userCenterUrl` 末尾追加 `?token=<platformToken>` 加载」,缓解只有 https + SPA 加载即 `history.replaceState` 抹除可见 URL。

经 grill 发现一处 spec 内部不一致:**04 §1.4 明令「敏感凭据禁止进入 query string(避免进入访问日志、代理缓存与 Referer)」**,全契约 GET 端点的凭据因此都走 `X-M5755-Platform-Token` 头。但 WebView 加载面却把**权限最高的** `platformToken`(能经 uc 面改密 / 换绑 → 账户接管)放进了 query——`replaceState` 只抹掉页面内**后续**可见 URL,挡不住那一次顶层文档 `GET userCenterUrl?token=xxx` **原样落进 uc SPA 那台 web 服务器及中间 CDN/代理的访问日志**,正是 §1.4 要防的。

## 决定

`userCenterUrl` 末尾改追加 **`#token=<platformToken>`(URL fragment)**,不再用 query。

- fragment(`#` 段)**根本不发往服务器** → 不进访问日志 / 代理缓存 / Referer,与 04 §1.4 同口径;
- 页面 JS 用 `location.hash` 读取 token、`history.replaceState` 抹除可见 fragment(沿用既有内存捕获);
- token 捕获在 hash 路由 `route()` **之前**执行并抹掉 token 段,故不与 uc SPA 的 `#/phone`、`#/orders` 等路由 fragment 冲突。

`/api/uc/v2/*` 维持 **Bearer(`X-M5755-Platform-Token` 头)** 鉴权不变(不改 ADR-0010 面)。

## 为什么不是请求头 / cookie

- **请求头(`loadUrl` 的 `additionalHttpHeaders`)行不通**:① 该头只搭载顶层文档首次请求,不会带到 SPA 自己发的 `/api/uc/v2` XHR;② 页面 JS 读不到「入站请求头」,SPA 无法取回 token 给后续 Bearer 请求用 → 数据面全挂。
- **JS Bridge 下发 token 行不通**:06/06a 把 bridge 死锁成仅 `postAccountAction` 一个方法、刻意移除了 `getAccountContext`;再加「下发账户上下文」方法等于把红线拆了。
- **请求头 + `Set-Cookie` 会话(更安全的 B 方案)代价过大**:要把 `/api/uc/v2` 鉴权从 Bearer 改成 cookie 会话,动 ADR-0010 面 + 平台后端;对一个「容器禁用项全关、origin 受控、只加载平台自家受控页」的 WebView,再挡 XSS 偷 token 的边际收益很小。
- **fragment 是性价比最高的修法**:近零成本补上 §1.4 想要的效果,守住现有 Bearer 契约。

## 考虑过的其他选项

- **维持 `?token=`(现状)** — 否决:违 §1.4,主账户令牌进平台侧访问日志。
- **`#token=` fragment** — **采纳**:不进服务器、JS 可读、与 hash 路由可共存(捕获先于路由)。残留:token 仍在页面 JS 内存里,理论上 XSS 可偷;但 uc 只加载平台自家受控页、origin 受控,此面被判可接受。
- **请求头 + Set-Cookie 会话** — 否决(理由见上);若未来威胁模型升级、需要 token 完全不进 JS,再走这条并立新 ADR。

## 后果

- **文档**:04 §2.1、06 §5、06a §7、07(加载源)、adr/0008 均改为 `#token=` 并注明「fragment 而非 query」的理由。
- **实现跟进(本 ADR 仅定口径,代码另改)**:
  - `uc/api.js` `captureToken`:现读 `searchParams.get('token')`,改读 `location.hash`、并把 token 段从 hash 抹掉(保证 `route()` 看到干净 hash);
  - SDK 侧 WebView 加载处:`?token=` → `#token=`;
  - `docs/assets/acceptance-uc/README.md` 等验收证据待代码改完后同步,不提前伪造。
- **不影响** `/api/uc/v2` 鉴权模型(仍 Bearer)、不影响 bridge 契约。
