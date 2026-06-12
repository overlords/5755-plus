# 07 UI 规格说明(SDK UI)

> 来源:旧项目 `/Users/x/Developer/m5755` 的 `sdk-ui/src/main/java/com/m5755/sdk/ui/SdkUi.java`(约 1261 行,UI 全部由 Java 代码内联构建)与 `SdkUiKit.java`(通用控件与颜色常量),并参考 `docs/sdk-ui-materials-prd.md`、`docs/sdk-ui-reference.md`、`docs/android-sdk-ui.md`。
> 本文档是新实现的唯一 UI 规格依据,所有结构、文案、颜色、尺寸均从旧代码逐行提取。截图引用位于 `assets/acceptance/`。

---

## 0. 强制约束(实现前必读)

### 0.1 技术栈约束

- `sdk-ui` **禁止**引入:AndroidX、support-v4、Kotlin、OkHttp/Volley 等第三方网络库、第三方支付原生 SDK、Fragment。
- 只允许使用 Android 平台 `View` / `WebView`,语言为 Java。
- `sdk-ui` 的 `debugRuntimeClasspath` 必须输出 `No dependencies`(零运行依赖)。
- 所有 UI 挂载在宿主 Activity 的 `android.R.id.content` 上的一个 `FrameLayout` 层(tag = `m5755_sdk_ui_layer`),不新增 Activity / Dialog / Fragment / Manifest 组件。

### 0.2 文案禁词约束(verifyMinimalUi 构建门禁)

UI 文案与可达入口中**不得出现**以下范围外业务词:**微信、支付宝、福利、客服、优惠券、代金券、消息(中心)、活动、礼包(范围外营销含义)、饭团、平台币** 等。当前最小化 AAR 只装配:协议告知、登录、设备验证、小号选择、实名防沉迷、维护门禁、SDK 自有支付容器、悬浮球用户中心入口、最小用户中心 H5 容器。

### 0.3 旧实现已知问题(新实现禁止照搬)

| 旧实现问题 | 位置 | 新实现要求 |
| --- | --- | --- |
| 写死默认小号 `DEFAULT_ACCOUNT_ID = "sub_9"`,小号列表为空时伪造一条 `sub_9|小号1:sub_9` | `parseSubAccounts` / `showSubAccountPicker()` 无参重载 | 小号一律来自服务端返回,不得凭空伪造;列表为空属平台侧异常(平台保障首个小号),按规格**阻断登录并输出诊断**,不渲染选择页 |
| 登录态校验写死令牌 `CURRENT_LOGIN_TOKEN = "token_5755_sample"` | `showSessionCheck` | 必须展示真实登录令牌(或脱敏后的真实令牌) |
| 角色上报结果写死样例数据(server_1 / 星河一区 / 云起 / 68 / 128000 / 12-6 / 328.00 / -1) | `showRoleReportResult` | 必须展示真实上报的角色字段 |
| 设备安全验证页的"发送验证码"按钮未绑定任何点击事件 | `showDeviceVerification` | 必须接入与登录页一致的验证码请求 + 60s 倒计时逻辑 |
| 空值占位用 `textOrDash` 输出字面 `"-1"` 直接显示给用户 | 全局 | `-1` 是协议占位值,界面层应转换为"—"或"未提供"等用户可读占位 |

### 0.4 视觉上游(SDK 设计系统)

视觉设计的上游工作区是 claude.ai 设计系统项目 **「5755 SDK Design System」**(projectId `9771b4b5-d20e-4245-880e-a5770c482588`),含设计令牌、规范卡、13 个组件(React/JSX,仅作视觉参考)与完整交互演示。治理关系(2026-06-12 决定):

- **本文档(07)仍是 Android 实现的唯一 UI 口径**;设计系统的产出要进入实现,必须先修订本文档,实现者只读本文档,不需要读 JSX。
- 设计系统的色板自本文档 §1.1 逐字提取,二者取值一致;设计系统新增的规范决定(语义颜色角色、圆角/阴影刻度、字体边界、动效立场)已并入 §1.9-§1.11。
- 设计系统的 React/CSS 代码**不进入** Android 工程(§0.1 技术栈约束);需要查看或拉取最新设计,经 DesignSync 工具按需读取,仓库不存快照。

---

## 1. 通用规范

### 1.1 颜色表(SdkUiKit 常量,全量)

| 常量名 | RGB | HEX | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 255,201,54 | `#FFC936` | 主色:主按钮、选中态、勾选、滚动条、H5 hero 区 |
| `PRIMARY_DEEP` | 243,173,18 | `#F3AD12` | 深主色:链接按钮文字、"添加小号"描边 |
| `TEXT` | 37,39,43 | `#25272B` | 主文字 |
| `MUTED` | 119,123,131 | `#777B83` | 次要文字 / 提示 |
| `WEAK` | 242,243,245 | `#F2F3F5` | 输入框底色、小号列表区底色 |
| `LINE` | 232,233,238 | `#E8E9EE` | 分割线、弱描边 |
| `DANGER` | 240,76,76 | `#F04C4C` | 危险/错误(常量保留,旧实现未直接使用) |
| `LINK` | 22,184,199 | `#16B8C7` | 链接色(常量保留,旧实现未直接使用) |
| `GREEN` | 55,191,98 | `#37BF62` | 成功色(常量保留,旧实现未直接使用) |
| `WHITE` | 255,255,255 | `#FFFFFF` | 卡片底色 |
| `DRAWER_BG` | 245,246,248 | `#F5F6F8` | 用户中心抽屉底色 |

SdkUi 内部局部色:

| 名称 | 值 | 用途 |
| --- | --- | --- |
| `SUB_ACCOUNT_ACTION_TEXT` | `#5D4300`(93,67,0) | 主按钮文字色、"当前登录"标签文字、进入箭头 tint |
| `SUB_ACCOUNT_LOGIN_BG` | `#FFF9DF`(255,249,223) | "当前登录"标签底色、状态弹窗状态条底色 |
| `SUB_ACCOUNT_CLOSE_TEXT` | `#A4A8B0`(164,168,176) | 小号页圆形关闭按钮"×"文字 |
| `SUB_ACCOUNT_CLOSE_BORDER` | `#DEE1E8`(222,225,232) | 小号页圆形关闭按钮描边 |
| `SMS_CODE_DISABLED_TEXT` | `#A6A9B0`(166,169,176) | 验证码按钮禁用态文字、输入框 hint、上限态"添加小号"文字 |
| 遮罩 | `#78000000`(黑 ≈47% 不透明) | 模态弹窗背后遮罩 |
| 次按钮底/文 | `#E7E9EF` / `#6B7078` | "拒绝""取消"按钮 |
| 协议勾选文案 | `#9A9CA3`(154,156,163) | "我已阅读并同意…" |
| 未选中 Tab 文字 | `#61646B`(97,100,107) | 登录 Tab |
| 详情正文 | `#4F535A`(79,83,90) | 状态弹窗 detail、实名限制条目 |
| 未勾选框描边 | `#D5D7DD`(213,215,221) | 协议复选框 |
| Toast 底 | `#B8000000` | 黑色半透明 toast |
| 轻提示底 | `#EEF7F8FA` | 默认小号自动进入提示条 |
| 悬浮球底/描边 | `#D62A303E` / `#2EFFFFFF` | 悬浮球 |
| 支付抽屉底 | `#F5F5F5`(245,245,245) | 支付页背景 |
| 支付标题 / SDK 徽标文 / 徽标描边 | `#111111` / `#898989` / `#9D9D9D` | 支付页头部 |
| 支付说明正文 | `#595E66`(89,94,102) | 支付说明卡 |
| 支付底栏底 | `#3F3F3F`(63,63,63) | 应付金额栏 |
| 确认支付按钮 | `#FF4962`(255,73,98) | 支付主操作 |
| 订单行 标签/值/金额 | `#222222` / `#6C6C6C` / `#676767` | checkoutRow 三列 |
| 用户中心关闭"×" | `#747880`(116,120,128) | 抽屉右上角 |

### 1.2 字号体系(sp)

| 字号 | 用途 |
| --- | --- |
| 10 | "默认"徽标文字、默认单选勾 ✓ |
| 11 | 小号页 "!" 信息圆标 |
| 12 | smallText(表单提示、协议文案)、"当前登录"标签、"SDK"徽标、协议勾选框 ✓ |
| 13 | hint 正文(弹窗说明文字,行距 +3dp)、黑色 toast、"添加小号"按钮 |
| 14 | 输入框文字、小号行名称(粗体) |
| 15 | 按钮默认(粗体)、Tab 标签(粗体)、悬浮球"账"(粗体) |
| 16 | "选择小号进入游戏"(粗体)、"支付说明"(粗体)、上限 toast(粗体)、实名成功标题(粗体)、订单行标签 |
| 17 | 主按钮文字、小号页顶部昵称(粗体) |
| 18 | 弹窗标题(粗体)、状态条文字(粗体)、自动进入提示(粗体)、小号上限说明弹窗正文与"我知道了" |
| 20 | "应付:…"、"确认支付" |
| 22 | "5755 游戏支付"标题、小号页"×"与"⇄"符号 |
| 26 | 用户中心关闭"×" |
| 28 | 支付页返回"‹" |
| 32 | 实名成功 ✓ 图标 |

smallText 行距 +2dp;hint 行距 +3dp。粗体使用 `Typeface.DEFAULT` + `Typeface.BOLD`。

### 1.3 弹窗容器(addModal)

- 白色卡片,圆角 **10dp**,elevation **18dp**,居中(`Gravity.CENTER`)。
- 宽度 = `min(指定宽度dp, 屏宽 - 48dp)`;各界面指定宽度:协议 520dp、登录 340dp、其余标准弹窗 420dp。
- 遮罩:全屏 `#78000000`,可点击(拦截穿透,但点击遮罩不关闭)。
- 标题区(可选):高 **52dp**,标题 18sp 粗体 `TEXT` 居中;可选右侧 26dp 圆形"×"关闭钮(白底、LINE 描边、文字 `#A5A8AF`,右边距 16dp)——旧实现所有公开界面均 **不带** 标题区关闭钮(`close=false`)。
- 内容区 padding:左右 24dp,顶部有标题时 4dp / 无标题时 18dp,底部 24dp。
- 无出入场动效(旧实现无动画;新实现可不加,如加需统一)。

### 1.4 按钮

- **通用按钮**(SdkUiKit.button):15sp 粗体、不强制大写、水平 padding 10dp、minWidth/minHeight 置 0、纯色圆角背景。
- **主按钮**(primaryButton):底色 `PRIMARY #FFC936`,文字 `#5D4300`,圆角 **6dp**,字号 **17sp**,标准布局:宽 MATCH_PARENT × 高 **48dp**,顶部外边距 16dp。
- **次按钮**(拒绝/取消):底色 `#E7E9EF`,文字 `#6B7078`,圆角 6dp,与主按钮同排时各占权重 1、高 48dp、中缝两侧各 10dp、顶部外边距 18dp。
- **链接按钮**(linkButton):透明底,文字 `PRIMARY_DEEP #F3AD12`,水平 padding 6dp。

### 1.5 输入框

- 底色 `WEAK #F2F3F5`,圆角 **6dp**,无描边;文字 14sp `TEXT`;hint 色 `#A6A9B0`;水平 padding 14dp;单行。
- 标准高度 **44dp**(fieldParams),字段间垂直间距 12dp(部分 14dp)。
- **内嵌按钮输入行**(inputWithButton):横向容器自身带 WEAK 底 + 圆角 6dp,内部 EditText 透明底占权重 1,右侧链接按钮固定宽 **104dp**(如"发送验证码""显示"),容器右 padding 10dp。
- 密码框 `inputType = TYPE_TEXT_VARIATION_PASSWORD`,右侧"显示"按钮(旧实现未绑定切换逻辑,新实现需实现明文切换)。

### 1.6 Toast(SDK 自绘,非系统 Toast)

- **黑色 toast**(showToast):文字 13sp 白色,底 `#B8000000` 圆角 6dp,高 40dp,水平 padding 16dp,位置:底部居中、距底 **86dp**,elevation 44,显示 **1600ms** 后移除;同时 `announceForAccessibility(message)`。
- **白色上限 toast**(小号上限):文字 16sp 粗体 `TEXT`,白底全圆角(999),minWidth 220dp,高 52dp,水平 padding 28dp,位置:屏幕中心向下偏 120dp,1600ms,带无障碍播报。

### 1.7 状态弹窗模板(showStatusModal)

登录态校验 / 角色上报 / 支付状态共用:

- 420dp 标准弹窗 + 标题。
- **状态条**:宽 MATCH_PARENT × 高 52dp,底色 `#FFF9DF` 圆角 8dp,文字 18sp 粗体 `TEXT` 居中。
- **详情**:13sp、色 `#4F535A`、行距 +3dp、顶部边距 14dp,多行键值对(`键:值` 每行一条)。
- 底部主按钮(48dp,顶边距 16dp)。

### 1.8 SDK 层挂载机制

- 单一 `FrameLayout` 层(tag `m5755_sdk_ui_layer`)加到宿主 `android.R.id.content`,复用且 `bringToFront()`。
- 模态界面层 `clickable=true`(拦截游戏触摸);悬浮球 / 自动进入提示层 `clickable=false`(游戏仍可操作)。
- `dismiss()`:清空层、`GONE`、取消验证码倒计时。

### 1.9 颜色语义角色(自设计系统并入,2026-06-12)

代码中的颜色引用优先使用语义角色命名(常量值与 §1.1 一一对应,不新增颜色):

| 语义角色 | 取值 | 说明 |
| --- | --- | --- |
| `ACTION_PRIMARY` / `ACTION_PRIMARY_TEXT` | `#FFC936` / `#5D4300` | 主操作底/字 |
| `ACTION_SECONDARY` / `ACTION_SECONDARY_TEXT` | `#E7E9EF` / `#6B7078` | 次操作(拒绝/取消)底/字 |
| `ACTION_PAY` | `#FF4962` | 支付确认(支付流唯一红色) |
| `TEXT_PRIMARY` / `TEXT_SECONDARY` / `TEXT_DETAIL` / `TEXT_DISABLED` | `#25272B` / `#777B83` / `#4F535A` / `#A6A9B0` | 文字四级 |
| `TEXT_LINK` | `#F3AD12` | 链接文字 |
| `SURFACE_CARD` / `SURFACE_SUNKEN` / `SURFACE_DRAWER` | `#FFFFFF` / `#F2F3F5` / `#F5F6F8` | 卡片/凹陷(输入底)/抽屉 |
| `BORDER_HAIRLINE` | `#E8E9EE` | 细分割线 |
| `STATUS_BAR_FILL` | `#FFF9DF` | 状态条/标签底 |
| `SCRIM` / `TOAST_BG` | `#78000000` / `#B8000000` | 遮罩/黑 toast |

### 1.10 圆角与阴影刻度(自设计系统并入)

- **圆角离散刻度(dp)**:3(小卡)/ 6(按钮、输入框)/ 8(状态条)/ 10(弹窗)/ 14(面板、订单卡)/ 16(H5 hero)/ 全圆(悬浮球、胶囊、头像、上限 toast)。新界面只从刻度中取值,不发明中间值。
- **阴影(elevation)刻度**:2(小号卡)/ 12(抽屉)/ 18(弹窗、面板)/ 44(toast)。低对比软阴影,无彩色阴影、无发光。

### 1.11 字体与动效边界(2026-06-12 决定)

- **字体**:Android 原生层使用**系统默认字体**(`Typeface.DEFAULT`),字重仅 regular/bold 两档;**不随 AAR 内嵌任何字体文件**(中文字体体积与最小化 AAR 原则冲突)。设计系统的品牌字体 HarmonyOS Sans SC 仅约束设计产物与未来用户中心 H5 页面,不约束 Android 原生层。
- **动效**:界面出现/消失无进出场动画(与旧实现一致);允许新增轻微淡入淡出,**禁止**弹跳、循环、装饰性动效。计时行为仅限:验证码 60s 倒计时、toast 1600ms、自动进入提示 1800ms。
- 按压态 = 底色变暗(不缩放);禁用态 = 去饱和金色或灰色文字。

### 1.12 屏幕方向适配(自设计系统并入,2026-06-12)

宿主游戏横竖屏皆有,SDK 层必须双向可用;方向判定 = 当前可用宽 < 高即竖屏,**旋转时实时切换形态**(监听配置变化,不重建业务状态)。各类界面的适配策略:

| 界面类型 | 横屏 | 竖屏 |
| --- | --- | --- |
| 居中弹窗(协议/登录/维护/状态弹窗)与小号选择面板 | 按各自宽度公式 | 同公式,靠 `min(…, 屏宽-48dp)` 自然收窄;不改形态 |
| 支付容器 | 右侧全高抽屉(§9) | **底部抽屉**:贴底、宽 = 屏宽、高 ≤ 屏高 80%、顶部圆角 16dp、顶部居中抓手条(40×4dp 圆角,`#CFD2D8`);关闭符号由"`‹` 返回"改为"`⌄` 收起"(§9) |
| 用户中心抽屉 | 左侧全高抽屉(§11.2) | 同为左侧全高抽屉,但宽度上限 **80% 屏宽**——必须保留右侧游戏可见条,维持"非全屏遮挡"语义 |
| 悬浮球 / 自动进入提示 | 不随方向改变形态 | 同左 |

- 宽度数值一律以本文档各节公式为准;设计系统网页演示中的固定像素宽(如 560px)是网页端简化,不作为实现依据。
- 旋转切换形态时:滚动位置可重置,但输入内容、勾选状态、倒计时等业务状态必须保留。
- **实现机制(2026-06-12 决定)**:状态保留依赖宿主游戏 Activity 声明 `android:configChanges="orientation|screenSize|smallestScreenSize|screenLayout"`(01 §6 游戏包规范,接入自检诊断型校验);SDK 层监听配置变化,仅做重新布局(弹窗宽度 clamp 重算、抽屉形态切换),不做 Activity 重建存活。

---

## 2. 协议告知页(showProtocol)

截图:`assets/acceptance/01_protocol.png`

- **用途与触发**:SDK 初始化后、登录前的个人信息保护引导;同意前不得进入登录。
- **布局**:遮罩 + 居中弹窗 **520dp** 宽,标题区 52dp,正文 hint(13sp `MUTED` 行距+3dp),底部双按钮行(次"拒绝" + 主"同意",各 48dp 高、权重 1、中缝 20dp、顶边距 18dp)。
- **文案**:
  - 标题:`个人信息保护引导`
  - 正文:`本游戏接入 5755 SDK。为提供游戏资源加载、联网、账号安全、实名防沉迷、支付、用户中心和诊断能力,SDK 需要处理必要的设备信息、网络信息、当前游戏小号信息和日志信息。`(空一行)`请阅读《用户注册协议》《用户隐私协议》《儿童隐私保护指引》《第三方信息共享清单》。同意后进入账号登录。`
  - 按钮:`拒绝` / `同意`
- **交互与回调**:
  - 拒绝 → `onClosed("protocol")` + 关闭层(游戏侧自行决定退出)。
  - 同意 → 关闭层 + `onProtocolAccepted()`(随后进入登录)。
- 新实现建议:四个协议名可做成可点击链接(旧实现为纯文本)。

---

## 3. 登录弹窗(showLogin)

- **用途与触发**:协议同意后进入;登录态失效"重新登录"也回到此页。
- **布局**(居中弹窗 **340dp** 宽,无标题区):
  1. **Tab 行**:高 46dp,两个等宽 Tab `验证码登录` / `密码登录`;Tab 文字 15sp,选中:`TEXT` 粗体 + 底部 **24dp×3dp** `PRIMARY` 圆角短下划线(距底 2dp);未选中:`#61646B` 常规、下划线隐藏。默认选中"验证码登录"。
  2. **表单区**:固定高 **174dp**,随 Tab 切换重建:
     - 提示行(12sp `MUTED`,高 30dp):验证码 Tab → `验证码用于登录,账号状态由平台识别`;密码 Tab → `可使用手机号或账号密码登录`。
     - 验证码 Tab:输入框 `请输入手机号`(44dp);内嵌按钮输入行 `请输入验证码` + 右侧 `发送验证码`(104dp,顶边距 12dp);空辅助行 40dp。
     - 密码 Tab:输入框 `请输入手机号码`;内嵌按钮输入行 `请输入密码`(密码型)+ `显示`;辅助行 40dp 右对齐链接 `忘记密码?`(36dp 高)。
  3. **登录按钮**:主按钮 `登录`,MATCH_PARENT×48dp,顶边距 16dp。
  4. **协议勾选行**(顶边距 12dp):圆形复选框 **18×18dp**(选中:`PRIMARY` 圆底 + 白色 ✓ 12sp 粗体;未选中:白底 + `#D5D7DD` 1dp 描边)+ 文案 `我已阅读并同意 用户协议 和 隐私政策`(12sp,色 `#9A9CA3`,左边距 8dp)。整行可点击切换勾选;默认 **未勾选**。
- **发送验证码流程**:
  - 手机号为空 → toast `请输入手机号`,不发请求。
  - 点击后按钮立即进入"发送中"态:禁用、文字 `发送中`、色 `#A6A9B0`;回调 `onSmsCodeRequested(account)`。
  - 结果由 `showSmsCodeRequestResult(message, devCode, success)` 驱动:
    - 成功 → 启动 **60 秒倒计时**(`SMS_CODE_COUNTDOWN_SECONDS = 60`):按钮禁用,文字依次 `60s`→`59s`→…→`1s`(每秒刷新,色 `#A6A9B0`),结束后恢复可用、文字 `重新发送`、色 `PRIMARY_DEEP`。
    - 失败 → 立即恢复为可用 `重新发送`。
    - toast:有调试验证码时 `调试验证码:{code}`;否则取 message,缺省 `验证码已发送` / `验证码发送失败`。
  - 切换 Tab 或重开登录页会停止并重置倒计时。
- **提交校验(顺序阻断)**:
  1. 未勾选协议 → toast `请先阅读并同意协议`(阻断)。
  2. 账号为空 → toast `请输入手机号或账号`。
  3. 凭证为空 → toast 按 Tab 取 `请输入验证码` / `请输入密码`。
  4. 通过 → 关闭层 + `onLoginSubmitted(method, account, credential)`(method ∈ SMS / PASSWORD)。
- 已知缺口(新实现需补):"显示"按钮未实现明文切换;`忘记密码?` 未绑定动作。

---

## 4. 设备安全验证(showDeviceVerification)

- **用途与触发**:设备首次使用账号密码登录时的绑定手机号短信验证。
- **布局**:居中弹窗 **420dp**,标题 `设备安全验证`;正文 hint `设备首次账号密码登录时,需进行绑定手机号短信验证。`;内嵌按钮输入行 `请输入验证码` + `发送验证码`(顶边距 14dp);主按钮 `提交`。
- **交互与回调**:提交 → `onDeviceVerified(code)`,随后进入小号选择页。
- **已知问题**:旧实现此页"发送验证码"未绑定监听、提交不校验空值;新实现必须复用登录页的发送/60s 倒计时逻辑,且验证码为空时阻断并 toast `请输入验证码`。

---

## 5. 小号选择页(showSubAccountPicker)

截图:`assets/acceptance/02_subaccount_picker.png`(列表)、`assets/acceptance/03_default_selected.png`(默认标记)

- **用途与触发**:登录成功后(无默认小号或主动切换小号时)选择进入游戏的小号。
- **数据**:`accountsPayload` 格式 `id|label,id|label,…`(label 缺省为 `小号:{id}`);另有 `defaultAccountId`(默认小号)、`currentLoginId`(当前登录小号)、`accountNickname`(5755 账号昵称,缺省 `5755玩家`)。**新实现:小号数据一律来自服务端;payload 为空属平台侧异常(平台保障首个小号),不渲染本页,由状态机阻断登录并输出诊断,不得伪造小号。**
- **容器(底部圆角面板,居中)**:
  - 白色面板,圆角 **14dp**,elevation 18,clipToOutline。
  - 宽度:`min( max(360dp, 屏宽-52dp), max(420dp, 目标宽) )`,目标宽 = 屏宽≥720dp 时 `屏宽-380dp`,否则 `屏宽-48dp`。
  - 高度:`min(430dp, max(320dp, 屏高-70dp))`;整体相对屏幕中心上移 22dp。
  - **圆形关闭按钮"×"**:42dp 圆形,白底、`#DEE1E8` 1dp 描边、文字 `#A4A8B0` 22sp,**骑跨面板右上角**(圆心对准面板角),elevation 22,contentDescription `关闭小号选择页`。
- **头部**(高 64dp,白底,水平 padding 24dp):
  - 左:5755 昵称(默认 `5755玩家`),17sp 粗体 `TEXT`,单行尾部省略。
  - 右:切换按钮 `⇄`,42×42dp,透明底、`MUTED` 22sp,contentDescription `切换5755账户`。
  - 下方 1dp `LINE` 分割线。
- **主体**(底色 `WEAK`,padding 24/12/24/8):
  - **标题行**(高 36dp):左 `选择小号进入游戏` 16sp 粗体 + `!` 信息圆标(18×18dp,白底 LINE 描边圆形,11sp 粗体 `MUTED`,左距 8dp);右 `添加小号` 按钮 **86×32dp**,白底圆角 8dp、文字 13sp:正常态文字与描边 `PRIMARY_DEEP`;**满 10 个**时文字 `#A6A9B0`、描边 `LINE`。
  - **小号列表**(ScrollView,行高 62dp,首行顶距 12dp、其余 6dp,列表右 padding 16dp;超过 3 条时右侧显示 3dp×70dp `PRIMARY` 圆角装饰滚动条,距顶 36dp、距右 3dp):
    - **小号行**(smallAccountItem):外层可点击;内层卡片高 48dp、顶距 14dp(给徽标留出叠放空间)、白底圆角 **3dp**、`LINE` 1dp 描边、elevation 2。
      - 名称:label,14sp 粗体 `TEXT`,左距 16dp。
      - `当前登录` 标签(仅当前登录小号):12sp 粗体 `#5D4300`,底 `#FFF9DF` 圆角 3dp,高 24dp,名称右侧 8dp。
      - 右侧进入箭头:20dp 圆形 `PRIMARY` 底 + 右箭头图标(`m5755_ic_chevron_right_24`,tint `#5D4300`),距右 8dp,contentDescription `进入`。
      - **默认徽标**(每行左上角叠放,偏移 左2dp/上8dp):白底圆角 6dp、LINE 描边、高 20dp、elevation 4;内含 14dp 圆形单选(选中:`PRIMARY` 圆底 + ✓ 10sp `#5D4300`;未选中:白底 LINE 描边)+ `默认` 文字 10sp `MUTED`。
- **文案清单**:`选择小号进入游戏`、`添加小号`、`当前登录`、`默认`、`已设置默认小号`(toast)、`最多添加10个小号哦`(上限 toast)、`1个游戏下最多创建10个小号,每个小号独立`(说明弹窗,原文含换行)、`我知道了`、contentDescription:`关闭小号选择页`/`切换5755账户`/`进入`。
- **交互与回调**:
  - 点小号行 → 记录当前小号 + `onSubAccountSelected(id)` → 进入登录态校验弹窗(valid=true)。
  - 点默认徽标 → `onDefaultSubAccountSelected(id)` + 以该 id 为默认重渲染本页 + toast `已设置默认小号`。
  - 点 `添加小号`:已满 10 个 → 白色上限 toast `最多添加10个小号哦`(见 1.6);未满 → `onAddSubAccountRequested()`;添加结果经 `showSubAccountAddResult(message, success)` 以黑色 toast 显示 message。
  - 点 `!` → 居中说明小弹窗:二级遮罩 `#66000000`;白卡圆角 8dp,宽 `min(330dp, max(300dp, 屏宽-220dp))`;正文 18sp `TEXT` 行距+6dp、padding 22/18、高 96dp;1dp 分割线;`我知道了` 18sp 粗体 `#5D4300` 高 42dp 居中,点击仅关闭该小弹窗。
  - 点 `⇄` → 关闭层 + `on5755AccountSwitchRequested()`(切换 5755 主账号)。
  - 点 `×` → 关闭层 + `onClosed("subaccount")`。
- **业务规则**:每个游戏下最多 **10 个**小号。

---

## 6. 默认小号自动进入提示(showSubAccountAutoEnter)

- **用途与触发**:老用户已设置默认小号时,登录后自动进入游戏,顶部弹出轻提示告知当前小号,并提供快速切换入口。
- **布局**:非模态层(无遮罩,游戏可操作)。顶部居中提示条:
  - 文案:`✓  当前小号:{accountId}    ⇄`(✓ 后两个空格、⇄ 前四个空格,单个 TextView)。
  - 样式:18sp 粗体 `TEXT`,底色 `#EEF7F8FA` 圆角 **16dp**,水平 padding 22dp,elevation 16。
  - 尺寸:宽 `min(520dp, 屏宽-64dp)` × 高 **82dp**,距顶 **44dp**,水平居中。
- **交互**:
  - 点击提示条 → 打开小号选择页(携带当前小号列表/默认/当前登录上下文)。
  - **1800ms** 后自动消失,消失后显示悬浮球。
- 无显式回调;切换动作经由小号选择页完成。

---

## 7. 实名认证页(showIdentityVerification)

- **用途与触发**:平台要求实名/防沉迷校验未通过时,登录链路中阻断弹出。
- **布局**:居中弹窗 **420dp**,标题 `防沉迷系统提示`:
  - 说明 hint:`根据国家相关规定,请完成实名认证;未成年玩家将受到游戏时长和支付限制。`
  - 限制条目(12sp,色 `#4F535A`,顶距 8dp、底距 16dp):`1. 部分游戏时间段将受到限制`(换行)`2. 游戏支付金额将受到限制`
  - 输入框:`请输入真实姓名`、`请输入身份证号`(垂直间距 12dp)。
  - 主按钮:`立即认证`。
- **交互与回调**:点击立即认证 → 关闭层 + `onIdentitySubmitted(realName, idNumber)`。旧实现不做本地校验;新实现应至少校验非空并 toast 阻断。
- **成功态**(showIdentitySuccess,小卡片):居中白卡圆角 8dp、宽 **230dp**、padding 22/26/22/18、内容水平居中;✓ 图标 **58dp** 圆形 `PRIMARY` 底 + 白色 ✓ 32sp 粗体;标题 `实名认证成功` 16sp 粗体(高 42dp,顶距 12dp);链接按钮 `我知道了`(高 40dp)→ 关闭并显示悬浮球。注:旧实现该方法为 private 且未被任何流程调用(死代码),新实现需把它接入实名提交成功回路。
- 失败态旧实现缺失;新实现需补失败 toast/弹窗并给出可操作原因。

---

## 8. 维护门禁页(showMaintenanceGate)

- **用途与触发**:平台返回游戏维护中,登录/进入链路阻断。
- **布局**:居中弹窗 **420dp**,标题 `维护门禁`;正文 hint = 服务端 message,为空时缺省 `当前游戏维护中,请稍后再试。`;主按钮 `我知道了`。
- **交互与回调**:我知道了 → `onClosed("maintenance_gate")` + 关闭层(不进入游戏)。

---

## 9. 支付弹窗(showGamePay)

- **用途与触发**:游戏调起充值下单后,展示 SDK 自有支付容器(右侧全高抽屉)。
- **入参**:accountId、productName、amountText、serverText、roleText、orderId(空值经 textOrDash 变 `-1`——新实现改为用户可读占位)。
- **布局**(遮罩 + 右侧抽屉):
  - 抽屉(横屏形态):宽 `min(屏宽, max(520dp, 屏宽/2))`,全高,贴右(`Gravity.RIGHT`);底色 `#F5F5F5`,padding 18/14/18/0,elevation 18。
  - **竖屏形态(§1.12)**:改为底部抽屉——贴底、宽 = 屏宽、高 ≤ 屏高 80%、顶部圆角 16dp、顶部居中抓手条(40×4dp,`#CFD2D8`);头部关闭符号 `‹` 改 `⌄`(语义"收起",回调一致);订单卡/支付说明/底部支付栏结构不变,正文区滚动、支付栏钉底。旋转时实时切换形态,订单业务状态保留。
  - **头部行**:返回 `‹`(链接按钮,28sp,44×48dp)| 标题 `5755 游戏支付`(22sp,`#111111`,居中,占权重 1,高 48dp)| `SDK` 徽标(12sp 粗体 `#898989`,48×28dp,透明底 + `#9D9D9D` 1dp 全圆角描边)。
  - **订单卡**(白底圆角 14dp,padding 20/10,内含 5 行 checkoutRow):
    - 行规格(高 44dp 三列):标签 16sp `#222222` 宽 76dp | 值 15sp `#6C6C6C` 权重 1 | 金额列 15sp 粗体 `#676767` 宽 112dp 右对齐。
    - 行内容:`商品`(值=商品名,金额列=金额)、`小号`、`区服`、`角色`、`订单号`(后四行金额列为空)。
  - **支付说明卡**(白底圆角 14dp,padding 18/14,顶距 16dp):标题 `支付说明` 16sp 粗体(高 34dp)+ 正文(13sp,色 `#595E66`):`当前页面只承载 SDK 自有支付流程。游戏内物品发放以游戏服务端收到并校验通过的充值回调为准,客户端通知只用于界面状态。`
  - **底部支付栏**(悬浮于滚动区底部,距底 18dp,高 **58dp**,底色 `#3F3F3F` 全圆角胶囊,elevation 10;滚动区底部预留 86dp):左 `应付:{金额}` 20sp 白色居中(权重 1)| 右 `确认支付` 按钮 **156dp** 宽、底 `#FF4962`、白字 20sp、无圆角(随胶囊裁切)。
- **交互与回调**:
  - 返回 `‹` → `onClosed("game_pay")` + 关闭层。
  - `确认支付` → `onPaymentRequested(SDK_CONTAINER, amountText)`,**不**自动关闭(等待支付状态回调驱动界面)。
- **边界约束**:不展示任何第三方原生支付方式;客户端支付回调不构成发货依据(文案已固化此口径)。

## 9b. 支付状态弹窗(showPaymentState)

截图:`assets/acceptance/05_payment_state.png`

- **用途与触发**:支付流程客户端状态回执(下单/拉起/回调结果)。
- **布局**:状态弹窗模板(1.7),标题 `客户端支付状态`。
- **状态条文案**(按 code):`2` → `支付处理中`;`0` → `支付已交接`;其他 → `支付未完成`。
- **详情**(逐行):
  - `订单号:{orderId}`
  - `金额:{amountText}`
  - `客户端状态:{message}`
  - `支付入口:未获取/已获取`(paymentUrl 为空或 `-1` → 未获取,否则已获取)
  - `游戏内物品发放以游戏服务端收到并校验通过的充值回调为准。`
- **交互**:`我知道了` → 关闭层 + 显示悬浮球。
- 另有内部"支付处理中"态模板(showPaymentProcessing,旧实现未接线):标题 `支付处理中`、状态 `等待服务端确认`、正文 `客户端支付通知只代表支付流程状态。游戏内物品发放必须等待游戏服务端收到 5755 充值回调并完成校验。`、按钮 `我知道了`。

---

## 10. 登录态校验弹窗(showSessionCheck)

截图:`assets/acceptance/04_session_check.png`

- **用途与触发**:选择小号进入游戏前、或游戏主动校验登录态时展示校验结果。
- **布局**:状态弹窗模板,标题 `登录态校验`。
- **状态条**:有效 → `登录态校验通过`;失效 → `登录态已失效`。
- **详情**(逐行):
  - `当前游戏小号 ID:{accountId}`
  - `登录令牌:{token}` ——**旧实现写死 `token_5755_sample`,新实现必须用真实令牌(建议脱敏)**
  - `登录态由游戏服务端按协议校验,SDK 只提供当前游戏小号和登录令牌。`
- **交互**:
  - 有效 → 按钮 `进入游戏`:关闭层 + 显示悬浮球。
  - 失效 → 按钮 `重新登录`:`onUserCenterAction("session_invalid")` + 回到登录弹窗。

## 10b. 角色上报结果弹窗(showRoleReportResult)

- **用途与触发**:游戏上报角色信息后展示上报结果与字段回显。
- **布局**:状态弹窗模板,标题 `角色上报`;状态条:成功 → `角色上报已完成`,失败 → `角色上报未完成`;按钮 `我知道了` → 关闭层。
- **详情字段**(逐行,**旧实现除角色 ID 外全部写死样例,新实现必须回显真实上报数据**;`-1` 为协议占位,界面应转可读占位):
  - `区服 ID:…`、`服务器名称:…`、`角色 ID:…`、`角色名称:…`、`角色等级:…`、`角色战力:…`、`角色关卡:…`、`累计充值:…`、`所属公会:…`
  - (旧实现写死值供对照,勿照搬:server_1 / 星河一区 / 云起 / 68 / 128000 / 12-6 / 328.00 / -1)

## 10c. 登出确认弹窗(showLogoutConfirm)

- **用途与触发**:用户中心"退出登录"或游戏退出流程触发确认。
- **布局**:居中弹窗 **420dp**,标题 `登出/退出`;正文 hint `确认后 SDK 会清理当前登录态,并通过账号变化路径通知游戏。`;底部双按钮行(同协议页规格):次按钮 `取消` + 主按钮 `确认退出`。
- **交互与回调**:取消 → 仅关闭;确认退出 → `onUserCenterAction("logout")` + `onClosed("logout")` + 关闭层。

---

## 11. 悬浮球 + 用户中心抽屉(showFloatBall / openUserCenter)

截图:`assets/acceptance/06_user_center_switch_subaccount.png`

### 11.1 悬浮球(addFloatBall)

- 非模态层。圆形 **54×54dp**,贴屏幕右侧,距顶 **138dp**、距右 **24dp**。
- 样式:TextView 文字 `账` 15sp 粗体白色居中;底 `#D62A303E`(深蓝灰 84% 不透明)全圆角 + `#2EFFFFFF` 2dp 描边;elevation 8。
- 点击 → 打开用户中心抽屉(悬浮球保留显示)。
- 旧实现固定位置不可拖动;参考文档建议未来支持"隐藏到边缘",当前版本不要求。

### 11.2 用户中心抽屉(openUserCenter)

- **容器**:左侧全高抽屉(`Gravity.LEFT`),宽 `min(屏宽, max(520dp, 屏宽×0.58))`,底色 `DRAWER_BG #F5F6F8`,elevation 12;层非模态(右侧游戏区域可操作)。**竖屏下宽度上限 80% 屏宽(§1.12)**——任何方向都必须保留右侧游戏可见条,不得全屏遮挡。
- **关闭按钮**:`×` 26sp、色 `#747880`,44×44dp,抽屉右上角(距顶 6dp、距右 8dp)→ `onClosed("user_center")` + 关闭层(含悬浮球;游戏侧可再调 showFloatBall)。
- **WebView**(铺满抽屉):系统 WebView;`javaScriptEnabled=true`、`javaScriptCanOpenWindowsAutomatically=false`、`allowFileAccess=false`、`allowFileAccessFromFileURLs=false`、`allowUniversalAccessFromFileURLs=false`;`overScrollMode=NEVER`;JS 桥名 **`UserCenter`**。
  - `loadDataWithBaseURL(null, html, "text/html", "UTF-8", null)`——**空 baseURL(不可信 origin)**,不得伪造平台域名;平台真实用户中心 H5 属未来范围,接入后改为加载真实 URL。
- **H5 内容规格**(本地最小容器,内联 HTML/CSS):
  - 字体栈:`-apple-system, BlinkMacSystemFont, 'PingFang SC', 'Microsoft YaHei', sans-serif`;body 底 `#f5f6f8`、文字 `#25272b`。
  - **Hero 区**:min-height 172px,底色 **`#ffc936`**,padding `28px 64px 24px 28px`;头像 64px 白色圆形(文字 `账`,border 3px rgba(0,0,0,.08));标签 `当前游戏小号 ID`(13px,rgba(0,0,0,.55))+ 小号 ID 加粗 26px;hero 提示(14px,rgba(0,0,0,.62),行高 1.7):`用户中心接收 SDK 当前游戏小号上下文,不维护另一套当前账号。`
  - **功能卡**(白色圆角 8px 卡片,上探叠入 hero:margin `-32px 18px 0`,阴影 `0 4px 18px rgba(0,0,0,.05)`):功能行高 58px、行间 1px `#f0f1f4` 分割、左右 padding 22px、文字 `#575b62` 加粗、行尾 `›` 箭头(`#a6a9b0`,26px)。当前仅两行:
    - `切换小号` → `postAction('switch_account')`
    - `退出登录` → `postAction('logout')`
  - **底部说明**(13px,`#9aa0a8`,margin 18px 22px 24px):`当前为最小化用户中心容器,仅提供小号上下文、切换小号与退出登录;更多账号服务由平台用户中心 H5 提供(后续接入)。`
- **JS 桥协议**:
  - `UserCenter.getAccountContext()` → 返回 JSON 字符串 `{"accountId":"…"}`(页面加载时同步刷新显示)。
  - `UserCenter.postAccountAction(action)` → 原生侧白名单归一化:仅接受 `logout` / `switch_account` / `session_invalid`,其余一律归一为 `unknown`;在 UI 线程回调 `onUserCenterAction(action)`。
- **边界**:用户中心不得扩展为通用 H5 容器(无文件上传、媒体选择、APK 下载安装、外部 App 跳转);功能行不得出现范围外业务(见 0.2)。

---

## 12. 回调接口总表(SdkUiCallback)

| 回调 | 触发点 |
| --- | --- |
| `onProtocolAccepted()` | 协议页点同意 |
| `onLoginSubmitted(method, account, credential)` | 登录页校验通过提交(method: SMS/PASSWORD) |
| `onSmsCodeRequested(account)` | 登录页/设备验证页点发送验证码 |
| `onDeviceVerified(code)` | 设备验证页提交 |
| `onSubAccountSelected(id)` | 小号选择页点小号行 |
| `onAddSubAccountRequested()` | 小号选择页点添加小号(未满) |
| `onDefaultSubAccountSelected(id)` | 小号行默认徽标点击 |
| `on5755AccountSwitchRequested()` | 小号选择页点 ⇄ 切换 5755 账户 |
| `onIdentitySubmitted(realName, idNumber)` | 实名页提交 |
| `onPaymentRequested(SDK_CONTAINER, amountText)` | 支付页确认支付 |
| `onUserCenterAction(action)` | 用户中心桥接动作(logout/switch_account/session_invalid/unknown)、登出确认、会话失效重登 |
| `onClosed(surface)` | 各界面关闭,surface ∈ protocol / subaccount / maintenance_gate / game_pay / logout / user_center |

状态展示型方法(由编排层驱动 UI):`showSmsCodeRequestResult`、`showSubAccountAddResult`、`showSessionCheck`、`showRoleReportResult`、`showPaymentState`、`showSubAccountAutoEnter`、`showMaintenanceGate`、`showFloatBall`。

---

## 13. 样例宿主参考(sample,非 SDK 范围)

截图:`assets/acceptance/07_sample_panel_scroll.png`

以下属 `sample` 演示工程(`SampleGameActivity.java`)的游戏侧 chrome,**不属于 SDK UI 交付范围**,仅供验收环境对照:

- 全屏横屏、程序绘制渐变游戏背景(蓝绿渐变 + 光斑 + 网格线)。
- 左上游戏标题 `凡人修仙传`(34sp 白粗体带投影);右上 `5755 SDK` 徽标;左上状态条(380×36dp 半透明深色);左下 `16+` 适龄方块(58dp);底部合规文案 `抵制不良游戏,拒绝盗版游戏。注意自我保护,谨防受骗上当。合理安排时间,享受健康生活。`
- 左侧 `SDK` 黄色开关按钮(82×34dp,`#FFC936`)展开可滚动场景面板(宽 178dp,深色半透明圆角),按钮项:新用户首登 / 老用户无默认 / 老用户默认小号 / 维护阻断 / 防沉迷阻断 / 切换小号 / 登录态校验 / 角色上报 / 游戏支付 / 用户中心 / 退出确认 / 登出(每项 30dp 高、黄底 `#FFC936` 圆角 6dp)。
- 支持 `adb shell am start … -e scene <值>` 直达场景:`protocol / login / device / subaccount / session / role / identity / maintenance / float / center / gamepay / callback / logout / default / anti_addiction`。
- 截图说明:样例面板、游戏背景与上述 chrome 出现在所有验收截图中,验收时只看 SDK 弹层部分。

---

## 14. 验收截图索引

| 截图 | 对应界面 |
| --- | --- |
| `assets/acceptance/01_protocol.png` | 协议告知页(第 2 节) |
| `assets/acceptance/02_subaccount_picker.png` | 小号选择页(第 5 节) |
| `assets/acceptance/03_default_selected.png` | 小号选择页·默认标记选中态(第 5 节) |
| `assets/acceptance/04_session_check.png` | 登录态校验弹窗(第 10 节) |
| `assets/acceptance/05_payment_state.png` | 支付状态弹窗(第 9b 节) |
| `assets/acceptance/06_user_center_switch_subaccount.png` | 用户中心抽屉·切换小号(第 11 节) |
| `assets/acceptance/07_sample_panel_scroll.png` | sample 场景面板(第 13 节,非 SDK) |
