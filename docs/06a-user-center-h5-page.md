# 06a 用户中心 H5 页面设计规格(平台远程页 uc SPA)

本文档是**用户中心 H5 页面自身**(平台远程单页,以下「uc SPA」,部署于 `userCenterUrl`,dev 占位 `uc.xingninghuyu.com`)的信息架构、导航、数据 API 与视觉口径。

边界划分:
- **容器与 bridge** 见 `06`(Android WebView 抽屉、`UserCenter.postAccountAction` 单方法);本文不重复,只引用。
- **视觉语言**取自设计系统「5755 SDK Design System」与 `07 §1`(色板、令牌、组件);uc SPA 用其中的 `ListRow`/`Badge`/`Button`/`Input`/`StatusBar` 组合,字体 **HarmonyOS Sans SC**(400/700,回退 Noto Sans SC → OS CJK)。
- **数据 API** 见 ADR-0010:独立「平台对玩家」面 `/api/uc/v2/*`,与 04 网关面并列,**不复用 `/api/sdk/v2`**。
- uc SPA **以主账户为核心**:玩家在此查看/管理自己的 5755 主账户;**主账户信息只对玩家展示,绝不经公开 API 或 bridge 透给游戏**(`account=游戏小号 ID` 的对游戏契约不变)。

---

## 1. 信息架构(分组设置表)

主页为单列「分组设置表」,自上而下:

| 区块 | 内容 | 交互 |
| --- | --- | --- |
| 身份头部 | 头像 + 主账户昵称 + 脱敏手机 + 实名徽标 | 只读 |
| 当前小号 | 当前游戏小号 label + `切换 ›` | 点 → `switch_account`(bridge) |
| 账号安全(分组) | `绑定手机`(值=脱敏手机,`›`)、`修改密码`(`›`)、`实名认证`(值=已实名/未实名,**只读无 `›`**) | 前两行 push 二级页;实名只读 |
| 我的(分组) | `充值订单`(`›`) | push 订单列表 |
| 退出登录 | 底部主按钮 | 点 → `logout`(bridge) |

- 账户注销**不进 v1**。
- 实名区块为**只读状态展示**(硬约束:bridge 无法触发 SDK 原生实名页,见 06 §3;不在页内自建 NPPA 提交):已实名显示 `已实名`;未实名显示 `未实名` + 说明文案,**不给「去实名」按钮**。

## 2. 导航模型

- 主页 → 二级页(换绑手机/修改密码/订单列表)为 **SPA 页内 push**:二级页自带顶部 `‹ 返回` 回主页。
- 抽屉右上角 `×`(SDK 容器提供,06 §11.2)**语义恒定 = 整体关闭用户中心**,与 SPA 内部层级无关;SDK 不向 H5 下发返回事件,H5 自管路由。
- 二级页**不再叠 sheet/弹层**;表单内联在二级页内。

## 3. 数据 API 面 `/api/uc/v2/*`

鉴权:`X-M5755-Platform-Token: <platformToken>`(Bearer 语义,复用 04 头);会话绑 `gameId` 三元组;`reason` 沿用 04 枚举。下表六端点已在服务端 `internal/api`(`api_uc.go`)实现,ADR-0010 面、不走 HMAC。

| 端点 | 方法 | 用途 | 关键返回/入参 |
| --- | --- | --- | --- |
| `/api/uc/v2/profile` | GET | 主账户身份 + 实名状态 + 当前小号 | `nickname`、`maskedPhone`、`avatarUrl`(可空)、`realNameStatus`(`verified`/`unverified`)、`currentSubAccount{account,label}` |
| `/api/uc/v2/orders` | GET | 充值订单列表(游标分页) | query `cursor`;返回 `orders[]{orderId,productName,amount,currency,createdAt,status}`、`nextCursor` |
| `/api/uc/v2/phone/sms-codes` | POST | 换绑:向**新手机号**发验证码 | body `newPhone`;复用 60s 倒计时口径 |
| `/api/uc/v2/phone` | PUT | 提交换绑 | body `newPhone`、`smsCode` |
| `/api/uc/v2/password/sms-codes` | POST | 改密:向**已绑手机**发验证码(短信验证身份) | 无 body(取会话绑定手机) |
| `/api/uc/v2/password` | PUT | 提交改密 | body `smsCode`、`newPassword` |

- **归属强校验(防横向越权/IDOR)**:所有 `/api/uc/v2/*` 端点的数据主体一律由 `platformToken` 解析出的 `platformAccountId`(绑 `gameId` 三元组)决定,**服务端不接受请求体/query 中的任何账户标识作为查询主体**;`orders` 的 `cursor` 仅作该账户订单集内的分页游标,不得跨账户寻址。越权/失效一律按 `platform_account_invalid` 或 401 收口,不回显他人数据,与网关面 04 §2.9.2 反探测口径对齐。
- **失效收口**:任一端点返回 `reason=platform_account_invalid` 或 401 → SPA 调 `postAccountAction("session_invalid")`,SDK 清理登录态回 5755 登录窗(06 §4),不在页内提示重试。
- **改密成功 → 强制重登**:改密使当前 `platformToken` 作废,SPA 提交成功后即调 `postAccountAction("session_invalid")`。
- **换绑手机成功不登出**:回主页刷新脱敏手机 + 成功 toast。新手机号已被占用 → `409` + `reason=param_invalid`(SPA 内联提示「该手机号已被占用」,不触发失效收口)。
- 金额一律真实货币 `currency=CNY`,前端渲染 `¥`;**订单字段不得出现平台币/代金券/优惠券等范围外业务**(07 §0.2)。

## 4. 主页布局

窄单列(抽屉宽 ≥520dp,竖屏 ≤80% 屏宽,06 §11.2),底色 `SURFACE_DRAWER #F5F6F8`,内容卡片 `SURFACE_CARD #FFFFFF`、圆角 14dp、elevation 18(`07 §1.10`)。整页可竖向滚动。

- **身份头部**(卡片,padding 20):左侧头像 48dp 圆形(`avatarUrl`,缺省=昵称首字 monogram,`PRIMARY #FFC936` 底 + `#5D4300` 字);右侧上行昵称 17sp 粗体 `TEXT_PRIMARY`,下行脱敏手机 13sp `TEXT_SECONDARY` + 实名 `Badge`(已实名=成功色 `GREEN #37BF62`「✓ 已实名」;未实名=`MUTED`「未实名」)。
- **当前小号**(分组卡):标题行 `当前小号` 13sp `TEXT_SECONDARY`;`ListRow` 值=小号 label、尾 `切换 ›`。
- **账号安全**(分组卡,组标题 `账号安全`):三条 `ListRow`(58dp,规格见设计系统 ListRow / `07 §1`):`绑定手机`(值=脱敏手机)、`修改密码`、`实名认证`(值=已实名/未实名,**去掉 chevron**,行不可点)。
- **我的**(分组卡):`ListRow` `充值订单`。
- **退出登录**:底部主按钮(`primaryButton` 规格,MATCH_PARENT×48dp);与卡片留 24dp 间距。

## 5. 二级页

通用框架:顶部条(高 48dp)`‹ 返回`(左,链接按钮)+ 居中标题;主体单列表单/列表;主操作为底部主按钮。

- **换绑手机**(标题 `换绑手机`):**顶部账号明示行**(脱敏手机 `正在为 139****1234 换绑手机`,防对错主账户操作,见 §5 末);说明 hint;`Input` 新手机号(11 位校验,口径同 `07 §3`);内嵌按钮输入行 验证码 + `发送验证码`(60s 倒计时);主按钮 `确认换绑`。成功 → 返回主页刷新 + toast `换绑成功`。
- **修改密码**(标题 `修改密码`):**顶部账号明示行**(脱敏手机 `正在为 139****1234 修改密码`,防对错主账户操作,见 §5 末);说明 hint(短信验证身份);内嵌按钮输入行 验证码 + `发送验证码`(发往已绑手机);`Input` 新密码(密码型 + 显示切换,**长度 8-32 位**,前后端同校验);主按钮 `确认修改`。**失败按后端 reason 显示具体原因**(如 `验证码错误`、`密码长度须 8-32 位`),非笼统「修改失败」。成功 → toast `密码已修改` → 调 `session_invalid` 回登录窗。
- **充值订单**(标题 `充值订单`):订单行列表(`CheckoutRow` 类比),每行 商品名 / `¥金额` / 时间 / 状态;游标分页(触底加载);空态文案 `暂无充值订单`。

> **账号明示行(防错位)**:改密 / 换绑两个「对当前主账户的写操作」二级页,顶部固定显示当前主账户**脱敏手机**(`正在为 139****1234 …`),让玩家提交前确认操作的是哪个主账户;数据复用主页 `profile.maskedPhone`(前 3 后 4,口径同 §4),脱敏**不放宽**(守 §7 全局脱敏口径)。缘起:一次账号错位排查——表现为「改密成功但新密码登录失败」,实为改密落在 A 主账户、登录用 B 主账户,密码从未「不匹配」。

## 6. 账号动作与 bridge

- 切换小号 → `UserCenter.postAccountAction("switch_account")`:SDK 进小号选择页(06 §4);H5 不关心结果。
- 退出登录 → 可先在 H5 弹二次确认(文案对齐 `07 §10c`),确认后 `postAccountAction("logout")`。
- 失效收口 → `postAccountAction("session_invalid")`(见 §3)。
- **bridge 兜底**:加载时 feature-detect `window.UserCenter`;不存在(如 dev 浏览器直开预览)时,切换/退出按钮置灰并提示 `请在游戏内打开`,数据区仍可只读浏览。

## 7. 安全

- 仅 https 加载;`platformToken` 经 **URL fragment**(`#token=`)传入(**不用 query**:`#` 段不发往服务器,不进平台侧访问日志/代理缓存/Referer,对齐 04 §1.4 凭据禁入 query),SPA **加载即用 `location.hash` 读入内存、`history.replaceState` 抹除可见 URL**,避免泄漏进历史/截图/日志。token 捕获在 hash 路由 `route()` 之前执行并抹掉 token 段,故不与 `#/phone`、`#/orders` 等路由 fragment 冲突。
- 所有 `/api/uc/v2/*` 调用带 `X-M5755-Platform-Token`;`origin`/CORS 受控(ADR-0010)。
- 容器禁用项(文件/媒体/APK/外跳/通用 H5)由 SDK 侧保持关闭(06 §2);uc SPA **不依赖**任何被禁能力。

## 8. 状态与文案

- **三态**:加载=骨架屏(卡片灰块);空订单=文案 + 留白;API 错误(非失效类)=页内 `重试` 入口。
- **文案清单**:`当前小号`、`切换`、`账号安全`、`绑定手机`、`修改密码`、`实名认证`、`已实名`、`未实名`、`我的`、`充值订单`、`退出登录`、`换绑手机`、`发送验证码`、`确认换绑`、`确认修改`、`正在为〔脱敏手机〕修改密码`、`正在为〔脱敏手机〕换绑手机`、`换绑成功`、`密码已修改`、`暂无充值订单`、`请在游戏内打开`、`重试`。
- **dev 联调验证码**:换绑/改密发码成功后,若响应含 `devCode`(仅 mock/dev),toast 显示 `调试验证码:〔code〕`——与 SDK 登录窗同口径(`SdkUi` 发码亦如此,见 04 `devCode` 约定);生产 `SmsData` 无 `devCode` 字段(`omitempty` + 仅 mock),自动退回 `验证码已发送`,不泄露。
- **禁词自检**(07 §0.2):全页**不得**出现 微信/支付宝/福利/客服/优惠券/代金券/消息(中心)/活动/礼包/饭团/平台币。订单、账号安全等区块均以真实货币与主账户原生概念表述。
