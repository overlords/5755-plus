# uc — 用户中心 H5 单页(uc SPA)

平台远程页,被 Android SDK 的「用户中心 H5 容器」(WebView 抽屉)加载,部署于 `userCenterUrl`(dev 占位 `uc.xingninghuyu.com`)。

**口径**:`docs/06a-user-center-h5-page.md`(信息架构/布局/文案/禁词)、ADR-0010(数据 API 面)、`docs/06`(容器与 bridge)。

## 技术栈

纯 HTML/CSS/JS,**无构建步骤**(与设计系统模板一致;页面简单,引框架属过度工程)。
- `index.html` — 壳
- `styles.css` — 令牌(取自设计系统 `tokens/colors.css` = 07 §1.1)+ 组件;字体 HarmonyOS Sans SC
- `api.js` — 数据层 + bridge 封装 + token 捕获/抹除
- `app.js` — 路由(hash)+ 视图(主页 / 换绑手机 / 修改密码 / 充值订单)

## 本地预览

```sh
cd uc && python3 -m http.server 8080
# 浏览器开 http://localhost:8080/?token=demo
```

独立浏览器里 `window.UserCenter` 不存在 → 顶部提示「请在游戏内打开」,切换/退出按钮置灰(数据区仍可只读浏览)。

## 数据 API

`api.js` 默认 `USE_MOCK = false`,走真接口:`/api/uc/v2/*` 六端点已实现(profile / orders / 换绑手机 / 改密),`BASE` 为绝对域 `https://sdk-dev.xingninghuyu.com/api/uc/v2`(ADR-0010 网络路径选②;靠服务端 CORS 放行)。本地无服务端预览时临时置 `true`;生产改 `https://sdk.xingninghuyu.com`。`real.*` 的 fetch 路径、`X-M5755-Platform-Token` 头、`platform_account_invalid`/401 → `session_invalid` 收口按 06a §3。

## 与 SDK 的契约边界

- 仅经 `window.UserCenter.postAccountAction(action)` 回传账号动作:`switch_account` / `logout` / `session_invalid`(06 §3)。
- **不**读取任何账户上下文(无 `getAccountContext`);主账户数据全部由本页凭 `platformToken` 自取。
- token 经 `?token=` 传入,加载即读入内存并 `history.replaceState` 抹除可见 URL(06a §7)。

## 测试

数据层纯逻辑单测(`node:test`,零依赖、无构建,不入部署):

```sh
node --test uc/api.test.js
```

覆盖:`captureToken`(token 捕获 + URL 抹除 + 无 token 守卫,06a §7)、`classifyResponse`(401 / `platform_account_invalid` → session_invalid 收口、普通错误、成功取 data,06a §3)。`api.js` 用 `typeof module` 双模导出:浏览器是 `const UC` 全局,Node 可 `require` 纯函数。
