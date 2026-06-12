# 新平台服务端使用独立子域名与 /api/sdk/v2 契约前缀

旧平台原型(U10)部署在 `dev.xingninghuyu.com` / `api.xingninghuyu.com`,其 `/api/sdk/v1/*` 接口有正在服务的外部使用方,不可扰动。我们决定:新平台服务端使用**独立子域名**(联调 `sdk-dev.xingninghuyu.com`,生产 `sdk.xingninghuyu.com`,DNS 与服务器控制权已确认),契约路径统一升为 **`/api/sdk/v2/*`**,04 文档已整体修订为 v2 口径。

理由:v1 永远属于 U10(历史与现行使用方),v2 属于新平台服务端,域名与路径双重隔离使谱系一眼可辨、共存无冲突;`sdk-` 前缀直说组件范围(SDK 网关面)。被否方案:在 dev 域名上按路径反代接管 `/api/sdk/v1/*`(扰动现行服务)、新端口(HTTPS 证书与移动网络对非 443 端口不友好)。

推论:里程碑 1(双端贯通切片)的会师点是 `sdk-dev.xingninghuyu.com` 上的 v2 init 端点;旧文档/README 中"对接 dev.xingninghuyu.com、init 已实测可用"的结论属 v1/U10,对新平台服务端不适用,v2 端点可用性须重新建立。
