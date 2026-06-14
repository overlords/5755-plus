# 08 验收标准与已知问题

本文是 5755 Android SDK 最小化 AAR 的权威验收规格,规定业务验收原则、验收前置条件、模拟器验收场景清单和当前已知问题。业务需求是否完成,以本文定义的验收面为判断依据。

## 1. 验收原则

业务需求以**样例游戏实际演示、诊断快照和上线阻断回归**三个验收面共同判断,缺一不可:

- 样例游戏(只接入最小化 AAR)中能被验收人员实际操作到的流程,是最高验收入口。
- 诊断快照(logcat tag `M5755Sdk`)验证状态机关键节点,证明流程不是只在 UI 上看起来正确。
- 上线阻断回归覆盖正常链路与异常链路,防止改动回退。回归通过但样例不可操作,视为验收不完整;样例可操作但回归缺失,须补齐回归后才能声明业务闭环完成。

v2 自始以真实平台数据驱动状态机:模拟器演示验收必须连接真实开发环境(`sdk-dev.xingninghuyu.com`,devCode 自给),不得用模拟平台、演示小号或本地成功兜底(见 `02`「模拟器演示验收」);UI 不得写死演示小号列表、演示订单或演示账号来表达业务流程;公开 API、状态机、字段校验、错误码和诊断输出必须真实可验收。

## 2. 验收前置条件

### 2.1 登录凭证

端到端完成 5755 账户登录,**必须具备平台测试账号,或后端 mock 模式返回的调试验证码(devCode)**。dev 后端对任意演示手机号不保证返回可用 devCode,也可能拒绝登录;无可用凭证时,验收只能覆盖到 5755 账户登录窗口为止,登录后链路(小号选择、角色上报、支付、用户中心)不可达。验收排期前须先确认测试账号或 devCode 可用。

### 2.2 环境校验

每次模拟器验收前必须确认运行的是当前有效验收包,避免历史包混入导致结果无效:

- 包名为 `com.m5755.sdk.ui.sample`;新用户首登场景须先清空该包数据。
- 日志 tag 为 `M5755Sdk`;初始化日志包含 `platformEnv=dev baseHost=sdk-dev.xingninghuyu.com gameId=<游戏ID>`。
- 登录链路日志为 `login_5755_account`;若出现其他 tag 或其他游戏前缀的账号日志,本次验收结果无效。
- 真实登录成功后,本地 `shared_prefs/m5755_operate_min.xml` 必须出现 `platform_account_id`、`platform_token` 和当前游戏小号 `account`。
- SDK UI 不表达"注册成功":口径为"5755 账户登录成功后进入实名检查",验证码登录下是否创建新平台主账户由服务端识别。

## 3. 模拟器验收场景清单

以下场景须在 Android 模拟器上以真实玩家路径逐一走通,截图归档于 `assets/acceptance/`:

| # | 场景 | 验收要点 | 证据 |
| --- | --- | --- | --- |
| 1 | 初始化与协议告知 | 冷启动先初始化成功,再展示协议告知;同意后进入登录页 | `assets/acceptance/01_protocol.png` |
| 2 | 5755 账户登录 | 验证码登录以 `sms` 提交,后端识别为登录链路 | 日志 `login submitted method=sms`、`login_5755_account` |
| 3 | 防沉迷实名 | 新账号登录后进入实名页,提交后返回实名通过 | 日志 `real_name required`、`real_name submit_success` |
| 4 | 真实小号选择 | 新用户实名通过后展示真实游戏小号列表,不出现演示小号 | `assets/acceptance/02_subaccount_picker.png`,日志 `sub_account real_list count=N` |
| 5 | 默认小号设置 | 点击小号行内默认标签后出现勾选状态,默认小号为当前选择 | `assets/acceptance/03_default_selected.png`,日志 `sub_account_default set` |
| 6 | 登录态校验与进入游戏 | 选择小号后返回当前游戏小号和登录令牌,登录态校验通过后游戏进入当前小号 | `assets/acceptance/04_session_check.png` |
| 7 | 默认小号自动进入 | 已设置默认小号后冷启动自动登录,展示轻量自动进入提示,不展示完整小号选择页 | 日志 `sub_account_auto_prompt default=<account>` |
| 8 | 角色上报 | 角色上报绑定当前游戏小号并返回成功 | 日志 `role_report state code=0` |
| 9 | 支付 | 支付页展示商品、当前小号、区服、角色和订单(均取自 `Order` 入参);确认支付后订单创建成功、支付入口已获取;支付状态弹层关闭后恢复悬浮入口 | `assets/acceptance/05_payment_state.png`,日志 `payment state code=0 ... paymentUrlSet=true` |
| 10 | 用户中心 | 悬浮入口打开 H5 用户中心,H5 为**平台远程页、以主账户为核心**(凭 `platformToken` 自取,不经 bridge 取小号上下文);切换入口文案为"切换小号"(不是"切换账号") | `assets/acceptance/06_user_center_switch_subaccount.png`(旧图为 #5 前样式,实测时刷新) |
| 11 | 用户中心切换小号 | 点击"切换小号"直接进入小号选择页,不触发防沉迷页、登出或 5755 账户登录页 | 日志 `user_center switch_account`、`sub_account_picker switch` |
| 12 | 样例操作面板 | 样例操作面板高度按屏幕可见区域约束,可滚动到角色上报、游戏支付、用户中心、退出确认和登出 | `assets/acceptance/07_sample_panel_scroll.png` |

异常路径回归(自动化门禁覆盖,作为上线阻断):网络失败、配置失败、渠道异常、维护阻断、协议拒绝、防沉迷进入游戏阻断、防沉迷支付阻断、支付失败、重复充值回调、5755 账户失效、游戏小号失效和用户中心账号动作,并确认维护/协议拒绝/防沉迷阻断均不触发账号变化。

模拟器验收通过后进入生产受控验收,后者单独覆盖真实资金通道、平台到游戏服务端充值回调、发货幂等、生产密钥和平台日志检索;模拟器验收不代表生产真实资金已通过,生产环境禁止运行会创建测试账号、小号、订单或调试支付完成的业务 smoke。

## 4. 旧实现已知缺口(B2–B5):不得带入 v2

以下 B2–B5 是**旧 SDK 参考实现**(`~/Developer/m5755`,纯 Java,模块 `sdk-core`/`sdk-ui`,旧契约 `/api/sdk/v1`)经代码核实的已知缺口。它们**不是 v2 的当前状态**:CLAUDE.md 核心不变量要求这四项缺口不得带入新实现,v2 从零重写时已正面规避并经审计坐实(下列「v2 现状」)。本清单保留作为验收人的"反面清单"——验收 v2 时须确认这些行为**不**出现。

> v2 是单一 `:sdk` 模块(ADR-0006),无 `sdk-core`/`sdk-ui` 模块、不调用 `/api/sdk/v1`。下文模块名/契约均指旧实现坐标。

### B2【P1】自动登录跳过服务端校验

- **旧实现本质**:`sdk-core` 的 `PlatformGateway` 自动登录分支读取本地 session 后即返回硬编码成功,未调用登录接口(旧契约 `/api/sdk/v1/login`)进行服务端校验。这违反"本地登录态只能用于发起自动登录,不能替代账户有效检查放行进入游戏;网络失败或平台不可用时不能用本地登录态放行"的规格。
- **v2 现状(已修复)**:`autoLoginOrWindow()` 必先 `gateway.checkAccount`(`GET /api/sdk/v2/account-sessions`)由服务端判定有效性——平台不可用→阻断(不用本地态放行)、失效→清本地态并回退 5755 账户登录窗口;无本地态直接放行的分支。单测 `autoLoginValidSkipsLoginWindow`/`autoLoginInvalidClearsAndShowsWindow`/`autoLoginPlatformUnavailableBlocksWithoutMisjudging` 覆盖。

### B4【P1】小号列表为空时伪造演示小号

- **旧实现本质**:`sdk-ui` 的 `SdkUi` 在小号列表为空时本地补一个演示小号(`sub_9`,默认账号常量 `DEFAULT_ACCOUNT_ID`)。这违反"小号选择页必须展示真实游戏小号列表、列表为空时阻断登录并输出诊断"的规格,造成 `account` 不可校验。
- **v2 现状(已修复)**:空列表一律 `showFlowBlocked('subaccount_list_empty', …)` 阻断并输出诊断,不本地伪造;全包搜 `sub_9`/`DEFAULT_ACCOUNT_ID` 无命中。单测 `emptyListBlocksWithoutFabrication` 覆盖。

### B3【P2】设备验证为空实现

- **旧实现本质**:`sdk-core` 的 `SdkRuntime.onDeviceVerified(code)` 只写一条诊断日志,不向后端校验设备验证码,任意字符串均视为通过。
- **v2 现状(已修复)**:设备首次密码登录返回 `reason=device_verification_required` → 完整设备验证 UI(发码 + 提交),`deviceId` 持久化生成,验证码经 `loginPassword` 带码续登发往服务端校验,非空壳。

### B5【P3】结果弹窗写死样例数据

- **旧实现本质**:`sdk-ui` 的角色上报结果弹窗写死样例区服/角色/等级/战力等字段,登录态校验弹窗写死样例登录令牌(`token_5755_sample`),展示内容并非实际上报值或真实令牌,具有误导性。
- **v2 现状(已修复)**:角色结果与支付订单卡逐字段取自入参(`reportRole`/`Order`),缺失字段渲染为「—」;支付确认仅提示"等待服务端充值回调"、不写死到账;全包搜 `token_5755_sample` 无命中。单测 `rechargeInvalidOrderNoRequest` 覆盖"无演示订单兜底"。
