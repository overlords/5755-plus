# 会话令牌 at-rest 明文存于 `MODE_PRIVATE` SharedPreferences 为 v2 显式范围决定

状态:已接受(2026-06-16)

## 背景

SDK 为支持自动登录(03 §2.4),把 5755 主账户会话持久化到 `shared_prefs/m5755_operate_min.xml`(`SharedPrefsStorage`,`MODE_PRIVATE`):

- `platform_account_id`、`platform_token`(**高权限主账户令牌**)、`account`、`sub_token`、`device_id`、`protocol_consented_version`——**全部明文**。

且 08 §2.2 验收口径**明文要求**这些键出现在该文件,以证明「真实登录态落地、非伪造」。即明文是为可验收**有意为之**。

经 grill(主题:玩家从登录到退出的信息泄露审计,威胁档位=抓包 + 本地取证)确认:此处不构成跨用户(轴①)或可越权(轴③)泄露——玩家读到的是**自己的**令牌,且令牌绑 `gameId` 三元组、无签名密钥不能伪造其他请求。但需把残留风险与威胁模型边界写清。

## 决定

**v2 接受会话令牌 at-rest 明文**,不引入 at-rest 加密。

判据:
- **最小化 AAR + 01 依赖白名单**:`EncryptedSharedPreferences` 会拉进 `androidx.security` + Tink,显著抬体积、可能违白名单;
- `MODE_PRIVATE` 已把文件沙箱到本 App UID,**非 root 的其他 App 读不到**;
- 存的是**玩家自己的**令牌,本机读取不构成跨用户泄露;
- 服务端有 `expiresAt` + 踢号(`platform_account_invalid`)可撤销,失窃令牌的有效窗口有界。

## 残留风险(写明,不藏)

设备**丢失 / 共享**、**root 恶意软件**或 **`adb backup`** 提取下,明文主账户令牌外泄 → 可经用户中心**改密 / 换绑 → 主账户接管**。此风险在「**威胁模型不含 root / 丢失设备**」前提下被接受。`sub_token` 外泄风险较低(仅游戏小号会话、绑游戏)。

## 后续硬化项(范围外,不阻塞 v2)

- **Android Keystore(AES/GCM)包裹** `platform_token` / `sub_token`:不引 `androidx.security` 重依赖即可防住设备取证 / 备份 / 部分 root;代价是要改 08 §2.2 验收断言(明文 → 密文/不可读)。
- 宿主 `allowBackup=false`:挡 `adb backup` 提取(注:库无法单方保证,需宿主游戏配合)。

威胁模型一旦升级到含 root / 丢失设备,优先做 Keystore 包裹并立新 ADR / 改本 ADR 状态。

## 考虑过的其他选项

- **明文(现状)** — **采纳**:零依赖、可验收、本机=自己令牌;残留风险写明并门控在威胁模型边界外。
- **Keystore 包裹双令牌** — 推迟为后续硬化:防护显著但要改验收断言,v2 威胁模型下收益不紧迫。
- **`EncryptedSharedPreferences`** — 否决:`androidx.security` + Tink 抬体积、可能违 01 依赖白名单与「最小化 AAR」。

## 后果

- **文档**:08 §2.2 增「at-rest 明文为 v2 显式范围决定 + 残留风险 + 后续硬化」说明,引用本 ADR。
- **代码**:`SharedPrefsStorage` 维持现状(明文 `MODE_PRIVATE`)。
- **验收**:08 §2.2 仍断言明文键存在(与现实现一致);未来若做 Keystore 包裹,需同步改该断言。
