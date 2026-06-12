package com.m5755.sdk.ui.sample;

import android.app.Activity;
import android.os.Bundle;
import android.view.Gravity;
import android.view.View;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;

import com.m5755.operate.api.Operate;
import com.m5755.operate.core.net.PlatformConfig;

/**
 * 样例游戏宿主(非 SDK 范围)。点击「进入游戏」模拟 onGameStart → 调 SDK 冷启动切片。
 */
public class MainActivity extends Activity {

    static final String GAME_ID = "m5755-demo";

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setGravity(Gravity.CENTER);
        int pad = dp(24);
        root.setPadding(pad, pad, pad, pad);

        TextView title = new TextView(this);
        title.setText("5755 SDK 样例");
        title.setTextSize(20);
        title.setGravity(Gravity.CENTER);
        root.addView(title);

        Button enter = new Button(this);
        enter.setText("进入游戏(onGameStart)");
        LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.WRAP_CONTENT, LinearLayout.LayoutParams.WRAP_CONTENT);
        lp.topMargin = dp(24);
        enter.setLayoutParams(lp);
        enter.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                // 验证用:intent extra baseUrl 指向本地/联调服务端(如 http://10.0.2.2:18080);
                // 缺省走包内 assets 配置(联调 sdk-dev)。
                String baseUrl = getIntent().getStringExtra("baseUrl");
                if (baseUrl != null && !baseUrl.isEmpty()) {
                    PlatformConfig local = new PlatformConfig(baseUrl, "local", "local",
                            "dev-test-key", "m5755-dev-public-test-secret-v1", "1.0.0");
                    Operate.get().start(MainActivity.this, GAME_ID, local);
                } else {
                    Operate.get().start(MainActivity.this, GAME_ID);
                }
            }
        });
        root.addView(enter);

        setContentView(root);
    }

    private int dp(int v) {
        return Math.round(v * getResources().getDisplayMetrics().density);
    }
}
