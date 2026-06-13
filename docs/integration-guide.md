# 5755 SDK v2 接入指南

面向游戏接入方的最小接入说明。口径以 `docs/01`(产品范围)、`docs/03`(主流程)、`docs/05`(支付/角色)、`docs/06`(用户中心)为准;本指南只覆盖**公开 API 白名单** `com.m5755.operate.api.*` 与 `com.m5755.operate.provider.OperateCode`。

## 1. 交付物

- **单一交付 AAR**:`sdk-prod-release.aar`(生产)/ `sdk-dev-release.aar`(联调)。一个 AAR 含全部能力,无需附加依赖(ADR-0006)。
- **零三方运行时**:交付 AAR 不含 AndroidX、kotlin-stdlib、OkHttp 等(`01 §4.1` 黑名单;由 `verifyPublicAarPurity` 门禁保证)。网络与 JSON 用 Android 平台内置。
- **不内嵌字体**(`07 §1.11`)。

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

承载 SDK 浮层的游戏 Activity **须**声明 `configChanges`,避免横竖屏切换时 Activity 重建、SDK 浮层连同输入/勾选/倒计时态一并丢失(07 §1.12「开屏即定」;未声明时接入自检输出诊断警告,不阻断):

```xml
<activity
    android:name=".MainActivity"
    android:configChanges="orientation|screenSize|smallestScreenSize|screenLayout" />
```

环境、渠道、签名由交付 AAR 的构建配置固定,**不通过参数透传或运行时切换**(`01 §5`)。

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
            // 用 account+token 到游戏服务端做登录态校验
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

所有字段必填;确无的字段传 `"-1"`,但 `roleId` 不允许 `"-1"`(`05 §1`)。

### 3.5 支付

```java
Order o = new Order();
o.setAmount(328.0);
o.setCpOrderId("P5755" + System.currentTimeMillis()); // CP 订单号,需唯一
o.setCommodity("648 元宝");
o.setServerId("s1");
o.setServerName("星河一区");
o.setRoleId("role_1");
o.setRoleName("云起");
o.setRoleLevel("68");
Operate.recharge(activity, o, new Listener() {
    public void onResult(boolean success, int code, String message) {
        // 仅 UI/状态提示,【不可】据此发货
    }
});
```

> **客户端支付回调 ≠ 发货依据。** 此处 `onResult` 只反映客户端支付交互结果。物品发放的**唯一依据**是平台→游戏服务端的**充值回调**(`05` 责任矩阵)。交付 AAR 不内置服务端验签/发放逻辑,发货幂等由游戏服务端负责。

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
Operate.destroy(activity); // 释放 SDK 资源,不改变账号状态(03 §6)
```

## 4. 公开错误码 `OperateCode`

SDK 对外只暴露这组粗粒度码(平台细分 `reason` 仅 SDK 内部分流,不外扩):

| 常量 | 值 | 含义 |
|---|---|---|
| `SUCCESS` | 0 | 成功 |
| `FAILURE` | 3 | 通用失败 |
| `NOT_INITIALIZED` | 6 | 未初始化或初始化未完成 |
| `NETWORK_ERROR` | 7 | 网络错误 |
| `TIMEOUT` | 8 | 请求超时 |
| `CANCELED` | 9 | 用户取消 |
| `PARAM_ERROR` | 10 | 入参非法 |

## 5. 接入红线

- 自动登录失败会**回退展示登录窗口**,不以失败码结束;接入方不要用本地登录态替代服务端校验放行。
- 游戏小号一律由平台返回,**不可凭空造演示小号**;列表为空属平台侧异常。
- 渠道异常一律回退 `default` 且不阻断登录。
- 诊断日志 tag = `M5755Sdk`。

## 6. 诊断

接入联调期可用 `adb logcat -s M5755Sdk` 观察 `init`/渠道解析/流程节点诊断输出。生产交付 AAR 不含任何 dev 控制面与测试桩(由发布门禁 `verifyPublicAarPurity` 五维 + 元测试保证)。
