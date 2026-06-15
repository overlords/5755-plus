# 设备验证 v2 默认关闭、每游戏可开;作用域 = 账户 × 安装(device_id 安装级随机、不跨游戏)

状态:已接受(2026-06-15)

## 背景

#25(密码登录新设备 SMS 验证)给密码登录加了「设备信任」第二因子:同一 5755 账户在**未信任的新设备**上密码登录时,须额外通过一次短信验证码(`deviceVerifyCode`)才能放行,验过即 `TrustDevice`、此后该设备免验。`domain.go authenticatePassword` 此前已收口为 **fail-closed**——`deviceId` 缺失即 `400 param_invalid`,堵「省略 deviceId 绕过设备校验」的 fail-open 弱点。

但 v2 是全新平台、首款游戏尚在适配,设备验证对**所有**密码登录强制开启会显著抬高接入摩擦(每个新设备多一次短信往返、SDK 必须每次携带 `deviceId`)。本 ADR 经 grill 决定:把整个设备信任块**门控在每游戏开关之后,版本默认关**,并顺带把「设备验证作用域」这条隐含口径写清——它由 v2「不指纹化物理设备」的隐私立场逼出。

## 决定

### (A) v2 默认关闭设备验证,每游戏可开

1. **每游戏开关**:`games` 表加 `device_verification_enabled boolean NOT NULL DEFAULT false`(migration `0015`,append-only,不回改 `0001`)。`default false` = **版本默认关、按游戏可开**;不存在全局开关。

2. **关(默认)= 密码登录纯密码**:密码 bcrypt 校验通过后,读该游戏开关;关则**跳过整个设备信任块、直接返回 `platformAccountId`**。`deviceId` 可带可不带、不强制(默认关下不存在 fail-closed 校验)。

3. **开 = 现有 fail-closed 设备块**:`deviceId` 必填(缺失即 `400 param_invalid`);设备未信任则返回 `device_verification_required`、要求 `deviceVerifyCode`(短信),验过即 `TrustDevice`。验证码爆破上限已由 `store.ConsumeSmsCode`(migration `0014`,设备码路径经 `ConsumeDeviceCode` 委托复用同一 per-code 上限)兜底。

4. **DB 读开关失败 → fault**:`503 platform_unavailable`(不静默放行、不静默拦截,符合「平台不可用须明确阻断」)。

5. **设备信任块代码与 `device_trust` 表 / `IsDeviceTrusted` / `TrustDevice` / `ConsumeDeviceCode` 全部保留**,只加门控——开关一旦置 `true`,行为与 #25 fail-closed 完全一致。

**安全取舍(写明,不藏)**:默认关之后,密码登录**无设备第二因子、无账户锁定**——纯靠 bcrypt 口令。这是「降接入摩擦」对「密码登录纵深防御」的有意让步;**高价值游戏可单独把 `device_verification_enabled` 置 `true` 拿回第二因子**。注意短信验证码登录路径**不受本开关影响**,其验证码爆破上限(`0014`)始终在线。

### (B) 设备验证作用域 = 账户 × 安装

设备信任记录键为 `(platform_account_id, device_id)`(`device_trust` 表)。其中 `device_id` 是 **SDK 安装级随机标识**:首次需要时在该 App 的私有 `SharedPrefs` 内生成一个随机值并持久化,卸载重装 / 清数据即重置。

由此得出作用域的两条推论:

- **不跨游戏**:每款接入游戏是独立 App、各有独立私有 `SharedPrefs`,`device_id` 互不相同。玩家在游戏 A 验过的设备,装上游戏 B 后 `device_id` 不同 → 视为新设备,**需重新验证**(在 B 也开启了设备验证的前提下)。
- **由隐私立场逼出**:跨游戏「一次验证、处处免验」需要一个**跨 App 稳定的物理设备标识**(OAID / Android ID 之类),而采集物理设备指纹属 **v2 范围外**(见 02「设备标识能力」与 `01` 能力白名单)。安装级随机 `device_id` 是「不指纹化物理设备」隐私决定的直接产物,跨游戏不可达是其必然代价,非疏漏。

## 为什么

- **默认关降接入摩擦**:首款游戏适配期,强制全量设备验证会让每个新设备多一次短信往返;默认关让接入方零配置即得「纯密码登录」,需要纵深防御时再按游戏开。
- **每游戏开关而非全局**:不同游戏价值 / 风险不同,高价值游戏单独开、低摩擦游戏保持关,粒度落在 `game_id` 与 NPPA 凭据 / serverKey 等其他每游戏配置一致(ADR-0007 / ADR-0016)。
- **保留而非删除设备块**:开关是配置操作、不是契约变更;`device_trust` 与 fail-closed 逻辑原样保留,置 `true` 即回到 #25 行为,改动可逆。
- **作用域写清**:`device_id` 安装级随机 → 不跨游戏,是隐私立场的推论而非缺陷;不写清会被误读为「设备信任失效」并被「补成跨游戏」的修复带偏到指纹化。

## 考虑过的其他选项

- **全局开关 vs 每游戏开关** — 选**每游戏**。全局单开关无法让高价值游戏单独开、低摩擦游戏单独关;每游戏粒度与既有 per-game 配置(NPPA 凭据 ADR-0007、serverKey ADR-0016)对齐。
- **默认开 vs 默认关** — 选**默认关**。默认开 = 全量接入方都吃设备验证摩擦、与「首款游戏适配期降摩擦」相悖;默认关把第二因子设为按需开启(opt-in),代价是写明的密码登录无第二因子取舍。
- **跨游戏设备信任**(一次验证、处处免验) — **否决**。需跨 App 稳定的物理设备标识(OAID / Android ID),即指纹化物理设备,与 v2「不采集物理设备指纹」隐私立场冲突(`01` 范围外);安装级随机 `device_id` 是隐私决定的产物,不跨游戏是其必然。

## 后果

- **migration `0015`**:`games` 加 `device_verification_enabled boolean NOT NULL DEFAULT false`(append-only)。dev seed 的 `m5755-demo` 默认关。
- **store**:新增读该游戏开关的途径(`SELECT device_verification_enabled FROM games WHERE game_id=$1`;DB 错 → error;无行 → `false` = 关)。
- **`domain.go authenticatePassword`**:bcrypt 校验通过后读开关 —— 关则直接 `return platformAccountId, nil`(跳过设备块);开则走现有 fail-closed 设备块;读开关失败 → `503 platform_unavailable`。
- **测试口径随之调整**:`TestPasswordLoginAndDeviceVerification` / `TestPasswordLoginMissingDeviceIDFailsClosed` 在 setup 里把目标游戏 `device_verification_enabled` 置 `true`(否则默认关让这俩失真);`api_uc_test.go` 改密后登录简化为纯密码登录(demo 默认关,贴默认行为);**新增**默认关下「正确密码 + 不带 `deviceId` → 密码登录成功(200 + 签发 `platformToken`)」锁定「默认关 = 纯密码」。
- **不动 SMS 爆破修复**:`store.ConsumeSmsCode` / `0014` 那桩与本 ADR 正交,保持不变。
- **文档对齐**:02 术语补「设备验证」词条(每游戏开关、v2 默认关、作用域账户 × 安装、`device_id` 安装级随机不跨游戏),与「设备标识能力」呼应。
