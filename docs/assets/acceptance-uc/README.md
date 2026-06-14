# 支轨 A · uc SPA dev 贯通留存证据

> 本目录是 roadmap「支轨 A」状态行所述「留存证据/acceptance 截图待补:合并前补」的兑现物。
> 记录 `user-center-h5` 合并 main 前,uc SPA(用户中心远程 H5 单页)对接 dev 平台真实数据面 `/api/uc/v2` 的 live 贯通证据。
> **uc SPA 四页正式截图验收**(主页/换绑手机/修改密码/充值订单)属支轨 A 自身、另行收口,见 #53。
>
> 口径:`docs/06a-user-center-h5-page.md`(SPA 信息架构/数据 API)、`docs/adr/0010-user-center-platform-api-surface.md`(Bearer 鉴权独立面)、`docs/06-user-center.md`(容器与 bridge)。

## 1. 主页真机实拍(已入库)

`uc-home-live.png` —— uc SPA 主页在 SDK WebView 容器内加载完成的真机截图(dev 真数据)。可见要点:

- **身份头部以主账户为核心**:玩家昵称 `玩家bd0810`、脱敏手机 `151****6175`、`✓ 已实名` 徽标 —— 凭 `platformToken` 真读 `GET /api/uc/v2/profile`,不经 bridge 取上下文(符合 `06` 容器边界、`06a §4`)。
- **当前小号区**:`小号1 · 切换 ›`(切小号经 bridge `postAccountAction`)。
- **账号安全分组**:绑定手机 `151****6175 ›`、修改密码 `›`、实名认证 `已实名`。
- **我的分组**:充值订单 `›`(`GET /api/uc/v2/orders`)。
- **退出登录**。
- 四页入口齐备、文案为「切换小号/退出登录」,无 `06a §8` 禁词。

## 2. 网关铸 token → uc 真读(live 走通)

- 网关侧用真实 dev 短信链路自给凭据铸主账户令牌:`POST /api/sdk/v2/sms-codes`(mock provider 返回 `devCode`)→ `POST /api/sdk/v2/account-sessions`(`loginMethod=sms`)→ 取 `platformToken`。
- uc SPA 凭 `?token=` 注入该令牌(加载即 `captureToken` 入内存 + `history.replaceState` 抹除可见 URL),`X-M5755-Platform-Token` 头真读 `profile` / `orders`,渲染如证据 1。
- `USE_MOCK=false`、`API_ORIGIN` 走绝对域(dev: `https://sdk-dev.xingninghuyu.com`)。

## 3. uc 真写 + 失效收口(live 走通)

- **改密真写**:`PUT /api/uc/v2/password`。live 验证「不发改密验证码 / 错码 + 合规密码」被后端拒(HTTP 4xx + `reason`);复核用新密登录失败 → 确认未误改。
- **前端失效/失败收口**:`classifyResponse` 按响应体 `success` 字段判定(非 `ok`,见修复 `1b52562`)——`401` 或 `success:false + platform_account_invalid` → `bridge.sessionInvalid()` 收口;其他 `success:false` → toast 具体 `error`,**不再误报「密码已修改」**。改密成功触发 `session_invalid` 强制重登。
- **换绑真写**:`PUT /api/uc/v2/phone`,回显新号、不登出。

## 4. 自动化/冒烟证据(已入库)

- `uc/api.test.js` —— 数据层纯逻辑单测 **8/8 通过**(`node --test`):`captureToken`(捕获/抹除/无 token 守卫)+ `classifyResponse` 失效收口五态(含针对 `1b52562` 的 4xx 回归守卫)。
- 渲染冒烟(改密/换绑页渲染出「正在为 `139****1234` 修改密码/换绑手机」账号明示行)与上述 live 改密 e2e 在 `user-center-h5` 开发期以 `.scratch` 探针脚本验证;结论转写于本记录(探针脚本属本地工作区,不入库)。

## 待收口(另行,不阻塞本次合并)

- **uc SPA 四页正式截图验收**:主页(本目录已具)+ 换绑手机 / 修改密码 / 充值订单 —— #53(ready-for-human)。
- **生产 uc 上线前置**:`uc/api.js` 的 `API_ORIGIN` 由 dev 切生产域 `https://sdk.xingninghuyu.com` —— 随主轴 GA 真平台一并落地,#56。
