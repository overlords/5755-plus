# SDK 收敛为单一 `:sdk` 源码模块以原生产出单一交付 AAR

里程碑 1 按旧项目结构把 SDK 拆成 `sdk-core` / `sdk-ui` / `sdk` 三个 Gradle 模块,`sdk` 仅作聚合(`api project(...)`)。但 AGP 不原生产出 fat-AAR:`sdk` 的 release AAR 只含聚合模块自身的类(约 3.4 KB),真实类留在两个传递 AAR 里,违背 01「接入方只依赖一个 AAR」。里程碑 4(生产化)需要真正的单一交付 AAR。我们决定:**把 core+ui+sdk 合并为单一 `:sdk` 源码模块**,AGP 原生 `assembleRelease` 即产出含全部类的单一 AAR。

被否方案:① 第三方 fat-aar 插件(`com.kezong:fat-aar` 等)——靠 hook AGP 内部 task,与 AGP 8.13 这类新版本耦合脆弱,绑架未来 AGP 升级;② 多模块 + 发布期 BOM 聚合——接入方实际仍依赖三个 AAR,违反 01 字面。

理由:core/ui/sdk 的 Gradle 模块边界对**接入方不可见**,其唯一作用是内部编译隔离;该隔离用 Java 包结构(`com.m5755.operate.core.*` 核心 / `com.m5755.sdk.ui.*` 业务配套 UI)+ `verifyPublicAarPurity` 的依赖断言同样可守住。合并换来 AGP 原生单一 AAR、零脆弱插件、面向未来 AGP 稳定——对长期维护的 SDK,稳定性远比模块墙值钱。

## Consequences

- 「SDK 核心模块」「业务配套 UI」由 Gradle 模块降级为**包结构逻辑分层**,职责边界不变;01 §1 / 02 术语「源码侧装配」措辞改为「单模块内按包分层」(随本 ADR 提交)。
- JVM 单测与 androidTest 仪器化测试迁入单 `:sdk` 模块;工程从四模块变两模块(`:sdk` + `:sample`)。
- 「生产不含 dev 能力」不再依赖模块隔离,而靠服务端 build tag(已有)与 `verifyPublicAarPurity` 的命名/依赖/Manifest 断言共同保证。
