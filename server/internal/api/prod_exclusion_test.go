//go:build production

package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"m5755/server/internal/domain"
	"m5755/server/internal/store"
)

// 生产构建排除复验(04 §3 三重防护①② + M4-S3):
// /internal/* 与 /pay/* 路由不存在;实名 mock 关闭时 fail-closed。

func TestDevControlAbsentInProduction(t *testing.T) {
	srv, _ := setup(t)
	resp, _ := doSigned(t, srv.URL, "GET", "/internal/dev-control/state", "gameId="+seedGame, nil, 0, false, false)
	if resp.StatusCode != 404 {
		t.Fatalf("生产构建 /internal/* 应 404,得 %d", resp.StatusCode)
	}
}

func TestPayPlaceholderAbsentInProduction(t *testing.T) {
	srv, _ := setup(t)
	resp, _ := doSigned(t, srv.URL, "GET", "/pay/P5755test", "", nil, 0, true, false)
	if resp.StatusCode != 404 {
		t.Fatalf("生产构建 /pay/* 应 404,得 %d", resp.StatusCode)
	}
}

// 生产口径(realNameMock=false):实名提交 fail-closed,不得以 mock 冒充核验。
func TestRealNameFailClosedWithoutProvider(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 DATABASE_URL")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	// SmsMock:true 让登录夹具走 mock 短信(本用例测的是实名 fail-closed,不是短信)。
	svc := domain.NewWith(st, domain.Options{CallbackSecret: "x", RealNameMock: false, SmsMock: true})
	srv := httptest.NewServer(NewRouter(svc, st, time.Now, "http://127.0.0.1:0"))
	t.Cleanup(srv.Close)

	_, paID, token, _ := loginNewUser(t, srv)
	rb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token,
		"realName": "张三", "idNumber": "11010119900101001X",
	})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/real-name", "", rb, nil)
	if ar.Success {
		t.Fatalf("未配置真实核验 provider 时实名提交应失败(fail-closed): %+v", ar)
	}
}
