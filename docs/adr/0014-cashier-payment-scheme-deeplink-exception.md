# 收银台为直连微信/支付宝破「最小化 AAR」禁外跳:支付域受限外跳例外

## 背景

#58 支付集成定为「平台自建收银台 + 微信/支付宝直连」(ADR-0012)。收银台是 AAR 内 WebView 加载的平台 H5 页;玩家在其中选微信/支付宝付款时,渠道 H5(微信 `h5_url` / 支付宝 wap)会 redirect 到 `weixin://`、`alipays://` 这类**非 http scheme**,拉起对应 App 完成付款。

而当前 AAR 的远程 WebView `shouldOverrideUrlLoading`(`SdkUi.loadableWeb`)只放行站内 http(s)、吞掉一切非 http scheme;`01 §4.2` 更把「外部 App 跳转」列为通用 H5 容器的**永久排除**。直连微信/支付宝**必然**要拉起 App,二者冲突。本 ADR 记录:为何为收银台破这条禁令,以及把破口收窄到何种程度。

## 决定

为**收银台支付流程**开「**支付域受限外跳例外**」,严格三重限定:

1. **仅收银台 WebView**:`loadableWeb` 增 `allowPaySchemes` 开关,只收银台挂载点传 `true`;协议网页层(§2)、用户中心(§11)及其余远程容器一律 `false`、行为零变化(仍吞掉非 http)。
2. **仅微信/支付宝白名单 scheme**:`weixin://`、`alipays://`(含已知付款变体如 `weixin://wap/pay`、`alipayqr://`);白名单外的非 http scheme 仍吞掉。
3. **仅 `startActivity(Intent.ACTION_VIEW)`**:不暴露任何公开 API、不引入微信/支付宝原生 SDK、不加 `<queries>`。

**未安装兜底走 `try/catch ActivityNotFoundException`(零 queries)**:Android 11 package visibility 只过滤「查询」(`resolveActivity`/`queryIntentActivities`),不阻止「直接启动」;故直接 `startActivity` 即可拉起、无需在 Manifest 声明 `<queries>`。App 未安装时捕获 `ActivityNotFoundException` → 原生层降级提示。提示文案**泛化、不点名渠道**(如「未检测到所选支付应用,请安装后重试或换一种支付方式」),使 AAR 原生层始终不出现「微信/支付宝」字样(守 `07 §0.2` 禁词)。

`01 §5` 评审结论:此例外**只触及 §4.2 一处**——§3 公开 API、§4.1 依赖黑名单、§4.3 Manifest(零 queries)、接入自检**均不破**。

## 为什么(破例的必要性 + 收窄理由)

- **必要**:ADR-0012 已定支付走持牌机构直连代收、不自持余额、不引原生 SDK。直连微信/支付宝 H5 支付的末端就是拉起 App(渠道侧强制,非我方可选);不外跳 = 不能用微信/支付宝 = 推翻 ADR-0012 的直连决议。
- **备选已否决**:(a) QR 码让玩家扫码到另一设备付款——断流、转化差,且 H5 支付本为「同机内拉起」设计;(b) 聚合支付/第三方收银 SDK——引入原生依赖、破 §4.1,且把资金链交给第三方,与 ADR-0012「持牌直连、平台不留沉淀」相悖。
- **为何零 queries(而非加 queries 预检)**:加 `<queries><intent scheme=weixin/alipays>` 能 `resolveActivity` 预检、把没装的渠道按钮预先置灰,体验略好;但会破 `01 §4.3`「必须排除……支付宝/微信 queries」,让「最小化 AAR」立项身份**再退一步(§4.2 + §4.3 两处)**。grill(2026-06-14)取**纯净优先**:try/catch 把侵蚀压到 §4.2 一处,代价仅是「玩家点了没装的渠道才提示」——收银台已有另一渠道兜底,可接受。

## 考虑过的其他选项

- **(a) 不破禁、改用 QR 扫码付** — 否决:断流、转化差,违直连 H5 设计。
- **(b) 引聚合支付/渠道原生 SDK** — 否决:破 §4.1 依赖黑名单 + 资金链旁落,撞 ADR-0012。
- **(c) 破禁 + 加 queries 预检** — 未选:体验略好但破 §4.3,立项身份多退一步。
- **(d) 破禁 + 支付域受限外跳 + try/catch 零 queries + 本 ADR** — **选中**:侵蚀仅 §4.2,Manifest 不动。

## 后果

- AAR 改动面小且收窄:`SdkUi.loadableWeb` 加 `allowPaySchemes` + 收银台白名单 scheme 外跳 + try/catch 兜底;Manifest **不变**(零 queries);公开 API **不变**;依赖**不变**。
- `01 §2`(支付行补注)、`§4.2`(外跳排除收窄)、`§4.3`(零 queries 加注)、`§5`(范围边界表「外部 App 跳转」拆出支付域例外)随本 ADR 编辑;`07 §9` 补收银台外跳态 + 泛化兜底文案。
- 微信 H5 支付要求 WebView 加载 `h5_url` 时带商户登记域 `Referer`——属平台商户后台配置 + 收银台 H5/容器实现细节,不增接入方配置项、不进 04 契约。
- 「支付方式不可知」基因(ADR-0012 前向兼容缝)不受影响:外跳是收银台 H5 → 渠道 App 的末端行为,SDK 仍只建方式无关 `Order`、收 3 态客户端回调。
