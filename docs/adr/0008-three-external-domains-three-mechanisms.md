# SDK 三类外部域用三种机制解析(API host / 协议域 / 用户中心)

SDK 会联系三类外部域:①平台网关 API host;②静态协议(法律页)域;③用户中心 H5。本轮变动新增了用户中心(#5,经 `/config` 下发)与协议链接站内网页层(#1,硬编码域),三者解析机制各不相同,读代码者易困惑「为何不统一」。本 ADR 固定三机制并记其所以然,防后期错乱。

决定:三类域**各用一种机制,刻意不统一**——

- **API host**(`baseHost`):按 flavor 写在构建配置 `m5755-sdk-platform.properties`(联调 `sdk-dev.xingninghuyu.com` / 生产 `sdk.xingninghuyu.com`)。必须按环境拆(dev/prod 是不同后端),构建期固定、运行时不可切换(环境不变量)。见 ADR-0004。
- **协议域**(`PROTOCOL_BASE`):SDK 内置硬编码常量 `https://p.xingninghuyu.com/agreement/`,各 flavor 同值。
- **用户中心 URL**(`userCenterUrl`):平台经 `GET /config`(04 §2.1)下发,按游戏配置;dev/prod 由「应答的平台部署」天然区分,SDK 不硬编码、仅末尾追加 `#token=<platformToken>`(**fragment 而非 query**,使主账户令牌不进平台侧访问日志/Referer;见 04 §2.1 / 06 §5 / 06a §7)。

理由(为何各异):
- API host 是后端本体,dev 与 prod 是真不同的服务器 → 必须按 flavor 拆 + 构建期固定。
- 协议域是静态法律页、跨环境同文,且**须在完整 `/config` 加载前就能展示**(新装即弹协议)→ 硬编码把协议显示与 config 拉取解耦,也不给它强加一个用不上的环境轴。
- 用户中心是动态平台页、按游戏且按环境,**不得硬编码**(06 §5)→ 由平台经 `/config` 下发,SDK 只附 token。

被否方案:
- 三域都走 `/config`:协议显示被迫依赖 config 拉取(破坏「新装即弹、早于 config」),并给无需环境轴的静态域强加环境轴。
- 协议域也进 properties 按 flavor 拆:可审计性略好,但协议页跨环境同值,徒增一个永不分叉的 flavor 维度;保留为常量更简,真要 dev 协议域时再迁不迟。

推论:
- 安全审计「SDK 联系哪些域」须看三处:properties(API host)+ SdkUi 常量(协议域)+ `/config` 响应的 `userCenterUrl`(用户中心)。
- 协议域硬编码**不违反**「环境/渠道/签名配置只能由构建/发布配置固定」——它是构建期固定常量,非运行时透传。
- 若将来协议页需分环境,把 `PROTOCOL_BASE` 迁入 properties(与 `baseHost` 并列)即可,契约不动。
