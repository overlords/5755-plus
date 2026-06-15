package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"m5755/server/internal/signature"
)

const (
	serverKeyID  = "dev-server-key"
	serverSecret = "m5755-dev-callback-secret-v1"
)

// TestServerKey_LookupAndVerify 验证 #84 / ADR-0016:
// 游戏服务端 serverKey(dev-server-key,principal=server)能被 LookupSigningKey 查到并验签通过;
// SDK keyId(dev-test-key)principal=sdk;未知 keyId 拒绝。
func TestServerKey_LookupAndVerify(t *testing.T) {
	srv, st := setup(t)
	ctx := context.Background()

	// 1) store 层:serverKey 查到 secret + principal=server + game_id=该游戏(grill 第 1 刀)
	secret, principal, gameID, ok, err := st.LookupSigningKey(ctx, serverKeyID)
	if err != nil || !ok {
		t.Fatalf("dev-server-key 应查到: ok=%v err=%v", ok, err)
	}
	if secret != serverSecret || principal != "server" {
		t.Fatalf("serverKey secret/principal 不符: secret=%q principal=%q", secret, principal)
	}
	if gameID != seedGame {
		t.Fatalf("dev-server-key game_id 应绑 %q, 得到 %q", seedGame, gameID)
	}

	// 2) SDK keyId principal=sdk、game_id 空(全局,区分主体)
	if _, sdkP, sdkGame, sdkOK, _ := st.LookupSigningKey(ctx, seedKeyID); !sdkOK || sdkP != "sdk" || sdkGame != "" {
		t.Fatalf("dev-test-key 应为 sdk principal、game_id 空: ok=%v principal=%q gameID=%q", sdkOK, sdkP, sdkGame)
	}

	// 3) 未知 keyId → ok=false
	if _, _, _, unknownOK, _ := st.LookupSigningKey(ctx, "no-such-key"); unknownOK {
		t.Fatal("未知 keyId 应 ok=false")
	}

	// 4) store.ServerKeyForGame 取该游戏出站签名用的最新 active serverKey
	skID, skSecret, skOK, skErr := st.ServerKeyForGame(ctx, seedGame)
	if skErr != nil || !skOK {
		t.Fatalf("ServerKeyForGame(%q) 应查到: ok=%v err=%v", seedGame, skOK, skErr)
	}
	if skID != serverKeyID || skSecret != serverSecret {
		t.Fatalf("ServerKeyForGame 选错密钥: keyId=%q secret=%q", skID, skSecret)
	}
	// 未配 per-game serverKey 的游戏 → ok=false
	if _, _, noOK, _ := st.ServerKeyForGame(ctx, "no-such-game"); noOK {
		t.Fatal("无 serverKey 的游戏应 ok=false")
	}

	// 4) 端点层:serverKey 签名调 config,验签通过(不应 signature_invalid)
	method, path := "GET", "/api/sdk/v2/config"
	query := "gameId=" + seedGame + "&sdkVersion=1.0.0&packageName=com.x.demo&channelId=&channelSource=default"
	ts := time.Now().Unix()
	req, _ := http.NewRequest(method, srv.URL+path+"?"+query, nil)
	for k, v := range signature.Sign(serverSecret, serverKeyID, method, path, req.URL.RawQuery, nil, ts) {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var ar apiResp
	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &ar)
	if ar.Reason == "signature_invalid" {
		t.Fatalf("serverKey 签名应验签通过、不应 signature_invalid;body: %s", b)
	}
}
