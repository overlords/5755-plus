# 升级 AGP 9 须随附两条接入方中和:排除 kotlin-stdlib + 压制 aarMetadata.minCompileSdk

状态:已接受(2026-06-16)

## 背景

Dependabot 提出把 AGP `com.android.application` / `com.android.library` 从 8.13.1 升到 9.2.1(#111 / #112)。一次深入验证(本地真构建 + AAR metadata 对比)发现:**AGP 9 不是 drop-in**,默认会引入两条**面向接入方**的回归,以及一条构建侧硬要求。

接入方面回归(若不处理,会抬高接入门槛、破坏交付承诺):

1. **kotlin-stdlib 注入运行时闭包**:AGP 9 的 library 插件把 `org.jetbrains.kotlin:kotlin-stdlib:2.2.10` 直接加入 `prodReleaseRuntimeClasspath`(`dependencyInsight` 实证:变体属性带 `AgpVersionAttr 9.2.1`,非本仓代码请求)。纯净门禁(`verifyPublicAarPurity`)当场 fail——撞 01 §4.1「零三方运行时」与 integration-guide §1「交付 AAR 无需附加任何依赖」。
2. **minCompileSdk 1 → 36**:AGP 9 默认把库的 `compileSdk(36)` 烙进 AAR `aar-metadata.properties` 的 `minCompileSdk`。AGP 8.13.1 下该值为 `1`(无约束),AGP 9 下变 `36`——**强制接入方 `compileSdk ≥ 36`(Android 16)**,否则消费 AAR 即构建失败。

构建侧硬要求:

3. **Gradle ≥ 9.4.1**:AGP 9.2.1 报「Minimum supported Gradle version is 9.4.1」,wrapper 8.13 必须升。

> kotlin-stdlib 是 JetBrains 官方库、本身正经;但「零三方运行时」承诺与依赖是否官方无关——纯 Java SDK 不需要 Kotlin 运行时,漏给接入方会造成:版本被迫抬升、Kotlin 元数据版本错位告警(老 Kotlin 编译器侧,`allWarningsAsErrors` 工程会变错误)、纯 Java 工程被塞 Kotlin 运行时、锁版本工程硬冲突。

## 决定

**升级 AGP 9 时,三处改动绑定随附,缺一不可:**

1. `android/gradle/wrapper/gradle-wrapper.properties`:Gradle `8.13 → 9.4.1`(AGP 9 硬要求)。
2. `android/sdk/build.gradle`:`configurations.configureEach { exclude group: 'org.jetbrains.kotlin', module: 'kotlin-stdlib' }`——剔除 AGP 9 注入的 kotlin-stdlib,守纯净门禁、不漏 Kotlin 运行时给接入方。
3. `android/sdk/build.gradle`:`defaultConfig.aarMetadata { minCompileSdk 21 }`——把 minCompileSdk 压回 21(=minSdk),不强制接入方 compileSdk。

**后两条是接入方保护红线,不得删除**:删 `exclude` → 接入方吃 kotlin-stdlib;删 `aarMetadata` → 接入方被迫 compileSdk≥36。本 SDK 为纯 Java、minSdk 21、不依赖 SDK-36 专属 API,两条中和均经验证安全。

## 为什么

- **接入方零新增强制是硬约束**:本 SDK 卖点之一是「自包含、零依赖 AAR」;升级构建工具链不应反向抬高接入门槛。加两条中和后,AGP-9 产出的 AAR 对接入方表现与 AGP-8 等价(`minCompileSdk=21`、`minAgpVersion=1.0.0` 不变、Java 8 字节码、无 kotlin-stdlib)。
- **证据驱动**:AAR metadata 对比(minCompileSdk 1↔36)、`dependencyInsight`(kotlin-stdlib 来自 AGP 9 注入)、纯净门禁(抓 kotlin-stdlib)三处实证,非推测。
- **纯净门禁的价值被印证**:它默默挡住了 AGP 9 的 kotlin-stdlib 注入——没有它,这条回归会静默进交付 AAR。

## 考虑过的其他选项

- **留在 AGP 8.13.1(不升)** — 可行的保守选择;但放弃 AGP 9 的修复与未来兼容性。本 ADR 选择「升、但带齐中和」。
- **升 AGP 9 不加中和** — 否决:破「零三方运行时」铁律 + 强制接入方 compileSdk≥36,直接抬高接入门槛。
- **`aarMetadata.minCompileSdk` 设 1(精确复刻 AGP-8)** — 未采;`21`(=minSdk)语义更诚实,且实践中任何消费 minSdk-21 库的工程 compileSdk 都远高于 21,等价于无约束。

## 后果

- 三文件改动随本升级一并合入;关闭 Dependabot #111 / #112(被本分支合并取代)。
- **残留弃用(非阻塞,单独跟进)**:① JDK21 对 Java 8 source/target 的弃用警告(AAR 仍发 Java 8 字节码、接入方无感,未来 AGP 会移除,需上 Java toolchain 或升 source/target);② Gradle 9 弃用项(Gradle 10 会破),需 `--warning-mode all` 定位。
- 本地验证(JDK21):`verifyPurityGateMetaTest` + `verifyPublicAarPurity`(五维过、`minCompileSdk=21`)+ `:sdk:test`(四变体)全绿。
