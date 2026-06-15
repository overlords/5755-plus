# 5755 SDK v2 接入指南

面向游戏接入方的最小接入说明。本指南**自包含**,只覆盖对外公开 API `com.m5755.operate.api.*` 与公开错误码 `com.m5755.operate.provider.OperateCode`,以及**游戏服务端对接**(§5:登录态校验 + 充值回调);接入方按本指南即可完成客户端接入与服务端对接。

## 1. 交付物

- **单一交付 AAR**:`sdk-prod-release.aar`(生产)/ `sdk-dev-release.aar`(联调)。一个 AAR 含全部能力,**无需附加任何依赖**。
- **零三方运行时**:交付 AAR 不含 AndroidX、kotlin-stdlib、OkHttp 等第三方库;网络与 JSON 用 Android 平台内置能力。
- 不内嵌字体。

集成(本地 AAR 示例):

```groovy
dependencies {
    implementation files("libs/sdk-prod-release.aar")
}
```

## 2. 权限与 Manifest

交付 AAR 只声明两项权限,接入方无需额外添加:

```xml
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
```

承载 SDK 浮层的游戏 Activity **须**声明 `configChanges`,避免横竖屏切换时 Activity 重建、SDK 浮层连同输入/勾选/倒计时态一并丢失(未声明时接入自检输出诊断警告,不阻断):

```xml
<activity
    android:name=".MainActivity"
    android:configChanges="orientation|screenSize|smallestScreenSize|screenLayout" />
```

环境、渠道、签名由交付 AAR 的构建配置固定,**不通过参数透传或运行时切换**。

## 3. 生命周期与公开 API

调用顺序:`onGameStart` → `init`(成功后才允许后续业务)→ `login` → 业务(`sendRoleInfo` / `recharge` / `changeUser`)→ 退出时 `shouldQuitGame` → `destroy`。下列示例与样例工程一致。

### 3.1 启动与初始化

```java
Operate.onGameStart(activity);
Operate.init(activity, new Options(GAME_ID), new Listener() {
    public void onResult(boolean success, int code, String message) {
        // success=true 后方可 login;否则按 code(见 §4)提示并允许重试
    }
});
```

`Options` 仅一个字段 `gameId`(无 OAID/AndroidID 透传、无环境切换扩展)。

### 3.2 账号变化监听(全局,登录前注册)

```java
Operate.setUserListener(new UserListener() {
    public void onLogout() {
        // 收敛入口:退出登录 / 切换小号 / 踢号 / 账户失效 / 小号失效
        // 维护门禁、协议拒绝、防沉迷门禁阻断【不】触发此回调
    }
});
```

### 3.3 登录

```java
Operate.login(activity, new DataListener<User>() {
    public void onResult(boolean success, int code, String message, User user) {
        if (success) {
            String account = user.getAccount(); // 当前游戏小号 ID
            String token = user.getToken();     // 小号登录令牌
            // 用 account+token 到游戏服务端做登录态校验(游戏服务端校验见 §5.1)
        }
    }
});
```

> `User.account` 恒为**当前游戏小号 ID**,不是 5755 平台主账户 ID;SDK 不向游戏暴露平台主账户 ID。`token` 仅来自小号登录,用于服务端校验。

### 3.4 角色上报

```java
RoleInfo r = new RoleInfo();
r.setServerId("s1");
r.setServerName("星河一区");
r.setRoleId("role_1");
r.setRoleName("云起");
r.setRoleLevel("68");
r.setRoleCe("128000");
r.setRoleRechargeAmount("328.00");
Operate.sendRoleInfo(r, new Listener() {
    public void onResult(boolean success, int code, String message) { }
});
```

所有字段必填;确无的字段传 `"-1"`,但 `roleId` 不允许 `"-1"`(无法提供唯一角色 ID 时不调用上报)。

### 3.5 支付

```java
Order o = new Order();
o.setAmount(328.0);
o.setCpOrderId("P5755" + System.currentTimeMillis()); // CP 订单号,游戏服务端生成,需唯一
o.setCommodity("648 元宝");
o.setServerId("s1");
o.setServerName("星河一区");
o.setRoleId("role_1");
o.setRoleName("云起");
o.setRoleLevel("68");
Operate.recharge(activity, o, new Listener() {
    public void onResult(boolean success, int code, String message) {
        // 见下:仅 UI/状态提示,【不可】据此发货
    }
});
```

> **客户端支付回调语义**:`onResult` 是**客户端支付流程的终态回调**,一次 `recharge` 只回调一次,在玩家关闭支付容器/收银台时触发:
> - `success=true`、`code=SUCCESS(0)` —— **已交接**:客户端支付流程结束、交回游戏,等待游戏服务端收到充值回调发货;
> - `success=false`、`code=CANCELED(9)` —— **未完成**:支付被取消或未确认;
> - 其它 `code` —— 失败(未登录/入参非法/门禁等,见 §4)。
>
> **`onResult` 仅用于 UI 提示与本地流程(如关闭等待框、引导重试),绝不是发货依据。** 物品发放的**唯一依据**是平台→游戏服务端的**充值回调**(§5.2)。`onResult=已交接` 也**不**代表已到账。交付 AAR 不内置任何服务端验签/发放逻辑。

### 3.6 切换小号 / 退出 / 销毁

```java
Operate.changeUser(activity, new DataListener<User>() {
    public void onResult(boolean success, int code, String message, User user) {
        // 取消切换保持当前小号(code=CANCELED)
    }
});

Operate.shouldQuitGame(activity, new OnQuitGameListener() {
    public void onQuit()   { /* 玩家确认退出,执行游戏退出 */ }
    public void onCancel() { /* 玩家取消 */ }
});

Operate.logout();          // 触发 UserListener.onLogout
Operate.destroy(activity); // 释放 SDK 资源,不改变账号状态
```

## 4. 公开错误码 `OperateCode`

SDK 对外只暴露这组粗粒度码(共 7 个,平台侧的细分原因仅 SDK 内部分流,不外扩):

| 常量 | 值 | 含义 |
|---|---|---|
| `SUCCESS` | 0 | 成功 |
| `FAILURE` | 3 | 通用失败 |
| `NOT_INITIALIZED` | 6 | 未初始化或初始化未完成 |
| `NETWORK_ERROR` | 7 | 网络错误 |
| `TIMEOUT` | 8 | 请求超时 |
| `CANCELED` | 9 | 用户取消 |
| `PARAM_ERROR` | 10 | 入参非法 |

## 5. 游戏服务端对接

游戏服务端有两件事对接 5755,都用**每游戏的 `serverKey`**(`serverKeyId + serverSecret`,5755 随接入材料下发)做 **HMAC-SHA256** 签名/验签(ADR-0016,与客户端 SDK 焊死的密钥分离;游戏服务端只学这一套签名)。机器可读契约见 `docs/server-facing-openapi.yaml`。

### 5.1 登录态校验(serverKey)

客户端 `login` 拿到的 `account`+`token`(§3.3)传到游戏服务端后,游戏服务端**必须向 5755 校验其真伪**——不能只信客户端传来的值(防伪造登录态):

- **接口**:`GET /api/sdk/v2/subaccount-sessions`(生产 `https://sdk.xingninghuyu.com`,联调 `https://sdk-dev.xingninghuyu.com`)
- **请求**:query `gameId`、`account`;header `X-M5755-Token`=玩家小号 `token`
- **鉴权**:用 `serverKey` 按 HMAC-SHA256 + 时间戳防重放签名,三个签名头 `X-M5755-Key-Id`(=`serverKeyId`)、`X-M5755-Timestamp`、`X-M5755-Signature`(算法见 `server-facing-openapi.yaml` / 对接材料)。`serverKey` **仅可调本端点**,调其他端点返 `principal_not_allowed`(403)
- **响应**:`data.valid=true` 才算登录态有效(HTTP 200 不等于放行);`data.account` 须与传入 `account` 一致,否则按无效处理

### 5.2 充值回调

**这是发货的唯一依据。** 客户端 `recharge` 的 `onResult`(§3.5)只表示客户端流程进展、不代表到账;玩家付款成功后,5755 平台向**游戏服务端**发起充值回调,游戏服务端据此发货。

- **方向**:5755 平台服务端 → 游戏服务端(接入方把回调地址提供给 5755 平台配置)。`HTTP POST`,JSON body。
- **回调字段**:

  | 字段 | 说明 |
  |---|---|
  | `account` | 当前游戏小号 ID(发货归属) |
  | `platformOrderId` / `order_id` | 平台订单号(`order_id` 为历史兼容别名,二者一致) |
  | `cpOrderId` / `cp_order_id` | 游戏订单号(下单时传入的 CP 订单号原样返回) |
  | `amount` / `money` | 订单金额(元) |
  | `pay_money` | 实付金额(元) |
  | `commodity` | 商品名称 |
  | `serverId` / `serverName` | 区服字段 |
  | `sign` | 平台 HMAC-SHA256 签名(口径见下「验签」;`serverSecret` 每游戏独立) |

- **验签**(HMAC-SHA256,ADR-0016):游戏服务端用每游戏的 `serverSecret` 复算 `sign`——取回调体除 `sign` 外全字段按字段名字典序升序、逐对拼 `键=值&`(含最后一对),以 `serverSecret` 为 **HMAC 密钥**对该串做 HMAC-SHA256、十六进制小写,与回调 `sign` 比对,不过一律拒绝。`serverSecret` 是 HMAC 密钥、**不拼进串**(取代旧 MD5);每游戏独立、随接入材料下发(dev 联调默认 `m5755-dev-callback-secret-v1`)。
- **金额 / 归属校验**:回调的 `amount` 须与游戏侧订单金额一致;`account` / `cpOrderId` 须与游戏侧订单记录归属一致。任一不符必须失败,不得发货。
- **幂等发货**:同一笔合法充值回调只发货一次。平台**可能重复发送**(网络抖动 / 投递重试 / 平台补偿巡检),游戏服务端须按 `cpOrderId`(或 `platformOrderId`)幂等去重,重复回调只确认、不重复交付。
- **响应**:游戏服务端处理成功须返回(平台据此止重推):

  ```json
  { "code": 200, "msg": "success" }
  ```

  返回其它内容或不可达,平台按策略重试 / 巡检重投,直至收到成功响应。

> 验签、金额校验、归属校验、幂等发货全部由**游戏服务端**实现(语言 / 框架不限);交付 AAR 不内置任何服务端发货逻辑。客户端 `onResult` 与本回调是两条独立链路,不得混用。

## 6. 接入红线

- 自动登录失败会**回退展示登录窗口**,不以失败码结束;接入方不要用本地登录态替代服务端校验放行。
- 游戏小号一律由平台返回,**不可凭空造演示小号**;列表为空属平台侧异常。
- 渠道异常一律回退 `default` 且不阻断登录。
- 发货只认充值回调(§5.2),客户端任何支付状态都不作发货依据。
- 诊断日志 tag = `M5755Sdk`。

## 7. 诊断

接入联调期可用 `adb logcat -s M5755Sdk` 观察 `init` / 渠道解析 / 登录 / 支付等流程节点的诊断输出(失败原因可区分:未登录 / 入参非法 / 网络 / 平台失败 / 防沉迷门禁)。生产交付 AAR 不含任何 dev 调试能力与测试桩。
