package com.m5755.sdk.ui.sample;

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;

/**
 * 样例游戏入口。里程碑 1 将在此 wiring onGameStart → init → login 的冷启动切片。
 */
public class MainActivity extends Activity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        TextView tv = new TextView(this);
        tv.setText("5755 SDK 样例");
        setContentView(tv);
    }
}
