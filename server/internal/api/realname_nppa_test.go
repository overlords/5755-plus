package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"m5755/server/internal/domain"
	"m5755/server/internal/nppa"
)

// 测试用 NPPA 凭据(secretKey 用官方示例值,确保 AES/签名合法)。
const (
	tAppID  = "fdc6688637bc468e9aea874654cbead2"
	tBizID  = "1101999999"
	tSecret = "b4c932250dbe59ff53b15ee993a9feb5"
)

// fakeNppa 同时处理 check(POST)与 query(GET):验证签名 + 解密请求,按配置返回 status。
func fakeNppa(t *testing.T, checkStatus, queryStatus int, gotAI *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts := r.Header.Get("timestamps")
		sign := r.Header.Get("sign")
		w.Header().Set("Content-Type", "application/json")
		writeResult := func(status int) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode": 0, "errmsg": "ok",
				"data": map[string]any{"result": map[string]any{"status": status, "pi": "1fffbjzos82bs9cnyj1dna7d6d29zg4esnh99u"}},
			})
		}
		if strings.HasSuffix(r.URL.Path, "/check") {
			body, _ := io.ReadAll(r.Body)
			want := nppa.Sign(tSecret, map[string]string{"appId": tAppID, "bizId": tBizID, "timestamps": ts}, string(body))
			if sign != want {
				t.Errorf("check 签名不匹配")
			}
			var env struct {
				Data string `json:"data"`
			}
			_ = json.Unmarshal(body, &env)
			plain, err := nppa.Decrypt(tSecret, env.Data)
			if err != nil {
				t.Errorf("check 请求体解密失败: %v", err)
			}
			var p struct{ AI, Name, IDNum string }
			_ = json.Unmarshal([]byte(plain), &p)
			if gotAI != nil {
				*gotAI = p.AI
			}
			if p.Name == "" || p.IDNum == "" {
				t.Errorf("check 明文缺字段: %s", plain)
			}
			writeResult(checkStatus)
			return
		}
		ai := r.URL.Query().Get("ai")
		want := nppa.Sign(tSecret, map[string]string{"ai": ai, "appId": tAppID, "bizId": tBizID, "timestamps": ts}, "")
		if sign != want {
			t.Errorf("query 签名不匹配")
		}
		writeResult(queryStatus)
	}))
}

func nppaServer(t *testing.T, checkStatus, queryStatus int, gotAI *string) *httptest.Server {
	t.Helper()
	st := smsTestStore(t)
	fake := fakeNppa(t, checkStatus, queryStatus, gotAI)
	t.Cleanup(fake.Close)
	if err := st.SetGameNppaCreds(context.Background(), seedGame, tAppID, tBizID, tSecret); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.SetGameNppaCreds(context.Background(), seedGame, "", "", "") })
	svc := domain.NewWith(st, domain.Options{
		CallbackSecret: "x", SmsMock: true, RealNameMock: false,
		NppaCheckURL: fake.URL + "/idcard/authentication/check",
		NppaQueryURL: fake.URL + "/idcard/authentication/query",
	})
	srv := httptest.NewServer(NewRouter(svc, st, time.Now, "http://127.0.0.1:0"))
	t.Cleanup(srv.Close)
	return srv
}

func nppaSubmit(t *testing.T, srv *httptest.Server, paID, token, name, id string) apiResp {
	t.Helper()
	rb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token,
		"realName": name, "idNumber": id,
	})
	return doSignedH(t, srv.URL, "POST", "/api/sdk/v2/real-name", "", rb, nil)
}

func nppaGetRealName(t *testing.T, srv *httptest.Server, paID, token string) apiResp {
	t.Helper()
	q := fmt.Sprintf("gameId=%s&platformAccountId=%s", seedGame, paID)
	return doSignedH(t, srv.URL, "GET", "/api/sdk/v2/real-name", q, nil, map[string]string{"X-M5755-Platform-Token": token})
}

// 成功:NPPA status=0 → 已实名;请求经签名+加密被 fake 正确解出 32 位 ai。
func TestRealNameNppaSuccess(t *testing.T) {
	var gotAI string
	srv := nppaServer(t, nppa.StatusSuccess, nppa.StatusSuccess, &gotAI)
	_, paID, token, _ := loginNewUser(t, srv)
	ar := nppaSubmit(t, srv, paID, token, "张三", "11010119900307001X")
	if !ar.Success || ar.Data["verified"] != true {
		t.Fatalf("应已实名: %+v", ar)
	}
	if len(gotAI) != 32 {
		t.Fatalf("ai 应为 32 位 hex,得 %q", gotAI)
	}
}

// 失败:NPPA status=2 → 提交失败,未实名。
func TestRealNameNppaFailed(t *testing.T) {
	srv := nppaServer(t, nppa.StatusFailed, nppa.StatusFailed, nil)
	_, paID, token, _ := loginNewUser(t, srv)
	ar := nppaSubmit(t, srv, paID, token, "张三", "11010119900307001X")
	if ar.Success {
		t.Fatalf("status=2 应失败: %+v", ar)
	}
}

// 认证中→懒查询定案:submit status=1(pending),GetRealName 时 query status=0 → 已实名。
func TestRealNameNppaPendingThenResolve(t *testing.T) {
	srv := nppaServer(t, nppa.StatusPending, nppa.StatusSuccess, nil)
	_, paID, token, _ := loginNewUser(t, srv)
	ar := nppaSubmit(t, srv, paID, token, "张三", "11010119900307001X")
	if !ar.Success || ar.Data["verified"] != false || ar.Data["pending"] != true {
		t.Fatalf("应认证中: %+v", ar.Data)
	}
	if ar.Data["antiAddictionEntryBlocked"] != true {
		t.Fatalf("认证中应阻进入: %+v", ar.Data)
	}
	g := nppaGetRealName(t, srv, paID, token)
	if g.Data["verified"] != true {
		t.Fatalf("懒查询后应已实名: %+v", g.Data)
	}
}

// 无凭据:game 未配置 NPPA → fail-closed。
func TestRealNameNppaNoCredsFailClosed(t *testing.T) {
	st := smsTestStore(t)
	_ = st.SetGameNppaCreds(context.Background(), seedGame, "", "", "")
	svc := domain.NewWith(st, domain.Options{CallbackSecret: "x", SmsMock: true, RealNameMock: false})
	srv := httptest.NewServer(NewRouter(svc, st, time.Now, "http://127.0.0.1:0"))
	t.Cleanup(srv.Close)
	_, paID, token, _ := loginNewUser(t, srv)
	ar := nppaSubmit(t, srv, paID, token, "张三", "11010119900307001X")
	if ar.Success {
		t.Fatalf("无 NPPA 凭据应 fail-closed: %+v", ar)
	}
}
