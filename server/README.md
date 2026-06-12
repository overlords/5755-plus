# 平台服务端(m5755/server)

5755 SDK v2 的平台服务端,实现 04 契约的 SDK 网关面。Go + gin + Postgres(pgx,手写 SQL),嵌入式版本化迁移在启动时套用。

里程碑 1 已实现:运维面(`/healthz`、`/openapi.json`)、入站验签(HMAC-SHA256)、`GET /api/sdk/v2/config`、`POST /api/sdk/v2/sms-codes`、`POST /api/sdk/v2/account-sessions`(sms 登录 + 首个小号保障建档)、dev 控制面(`maintenance`/`reset`/`state`)。

## 本地运行

```bash
export DATABASE_URL='postgresql://USER:PASS@HOST:5432/m5755_v2'   # 不入库
export ADDR=':8080'
go run ./cmd/server          # 启动时自动套用迁移 + 种子
```

种子:测试游戏 `m5755-demo`、dev 公开测试签名密钥 `dev-test-key`(secret `m5755-dev-public-test-secret-v1`,故意非机密,仅联调/回归用)。

## 测试(HTTP-seam,对真实 Postgres)

```bash
export DATABASE_URL='postgresql://USER:PASS@HOST:5432/m5755_v2'
go test ./...                       # dev 构建:dev 控制面路由存在
go test -tags production ./...      # 生产构建:断言 /internal/* 返回 404
```

未设 `DATABASE_URL` 时测试自动跳过。

## 构建

```bash
go build ./...                      # dev 构建(注册 dev 控制面)
go build -tags production ./...     # 生产构建(/internal/* 路由不注册)
```

## 部署

dev 部署到 CTID 105(`sdk-dev.xingninghuyu.com`),生产到 CTID 106(`sdk.xingninghuyu.com`):

```bash
scripts/deploy.sh dev      # 交叉编译 linux/amd64 → 推送 → systemd 重启 → healthz 检查
scripts/deploy.sh prod     # -tags production
```

部署配置放 `scripts/.env.deploy`(已 gitignore);容器内 systemd 环境文件 `/opt/m5755/.env` 提供 `DATABASE_URL`(密钥不入库)。systemd 单元见 `scripts/m5755-server.service`。

## 签名(04 §1.3)

每个 `/api/sdk/v2/*` 与 `/internal/dev-control/*` 请求需带:
`X-M5755-Timestamp`、`X-M5755-Key-Id`、`X-M5755-Signature`(HMAC-SHA256)。
canonical = `方法\n路径\n字典序query\n时间戳\n请求体`(GET 体为空串);时间戳窗口 ±300s。
GET 凭据走 `X-M5755-Platform-Token` / `X-M5755-Token` 请求头,不进 query。
