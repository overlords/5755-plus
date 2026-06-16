package com.m5755.sdk.ui.sample;

import android.app.Activity;
import android.os.Bundle;
import android.view.Gravity;
import android.view.View;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Toast;

import com.m5755.operate.api.DataListener;
import com.m5755.operate.api.Listener;
import com.m5755.operate.api.OnQuitGameListener;
import com.m5755.operate.api.Operate;
import com.m5755.operate.api.Options;
import com.m5755.operate.api.Order;
import com.m5755.operate.api.RoleInfo;
import com.m5755.operate.api.User;
import com.m5755.operate.api.UserListener;

/**
 * 样例游戏宿主(非 SDK 范围)。操作面板驱动公开 API:init → login → 用户信息/切换/登出。
 */
public class MainActivity extends Activity {

    static final String GAME_ID = "m5755-demo";

    private TextView status;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        Operate.setUserListener(new UserListener() {
            @Override
            public void onLogout() {
                runOnUiThread(new Runnable() {
                    public void run() {
                        setStatus("账号变化:已登出/失效");
                    }
                });
            }
        });

        ScrollView scroll = new ScrollView(this);
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setGravity(Gravity.CENTER_HORIZONTAL);
        int pad = dp(20);
        root.setPadding(pad, pad, pad, pad);
        scroll.addView(root);

        TextView title = new TextView(this);
        title.setText("5755 SDK 样例");
        title.setTextSize(20);
        title.setGravity(Gravity.CENTER);
        root.addView(title);

        status = new TextView(this);
        status.setText("未初始化");
        status.setTextSize(13);
        status.setGravity(Gravity.CENTER);
        status.setPadding(0, dp(8), 0, dp(8));
        root.addView(status);

        addButton(root, "进入游戏(onGameStart + init)", new Runnable() {
            public void run() {
                Operate.onGameStart(MainActivity.this);
                Operate.init(MainActivity.this, new Options(GAME_ID), new Listener() {
                    public void onResult(final boolean success, int code, final String message) {
                        runOnUiThread(new Runnable() {
                            public void run() {
                                setStatus(success ? "init 成功,可登录" : "init 失败:" + message);
                            }
                        });
                    }
                });
            }
        });

        addButton(root, "登录(login)", new Runnable() {
            public void run() {
                Operate.login(MainActivity.this, new DataListener<User>() {
                    public void onResult(final boolean success, int code, final String message, final User user) {
                        runOnUiThread(new Runnable() {
                            public void run() {
                                setStatus(success
                                        ? "登录成功 account=" + user.getAccount()
                                        : "登录未完成:" + message);
                            }
                        });
                    }
                });
            }
        });

        addButton(root, "用户信息(getUser)", new Runnable() {
            public void run() {
                User u = Operate.getUser();
                toast(u == null ? "未登录" : "account=" + u.getAccount());
            }
        });

        addButton(root, "切换小号(changeUser)", new Runnable() {
            public void run() {
                Operate.changeUser(MainActivity.this, new DataListener<User>() {
                    public void onResult(final boolean success, int code, final String message, final User user) {
                        runOnUiThread(new Runnable() {
                            public void run() {
                                setStatus(success ? "已切换 account=" + user.getAccount() : message);
                            }
                        });
                    }
                });
            }
        });

        addButton(root, "角色上报(sendRoleInfo)", new Runnable() {
            public void run() {
                RoleInfo r = new RoleInfo();
                r.setServerId("s1");
                r.setServerName("星河一区");
                r.setRoleId("role_1");
                r.setRoleName("云起");
                r.setRoleLevel("68");
                r.setRoleCe("128000");
                r.setRoleRechargeAmount("328.00");
                Operate.sendRoleInfo(r, new Listener() {
                    public void onResult(final boolean success, int code, final String message) {
                        runOnUiThread(new Runnable() {
                            public void run() {
                                setStatus(success ? "角色上报成功" : "角色上报失败:" + message);
                            }
                        });
                    }
                });
            }
        });

        addButton(root, "游戏支付(recharge)", new Runnable() {
            public void run() {
                Order o = new Order();
                o.setAmount("328.00");
                o.setCpOrderId("P5755" + System.currentTimeMillis());
                o.setCommodity("648 元宝");
                o.setServerId("s1");
                o.setServerName("星河一区");
                o.setRoleId("role_1");
                o.setRoleName("云起");
                o.setRoleLevel("68");
                Operate.recharge(MainActivity.this, o, new Listener() {
                    public void onResult(final boolean success, int code, final String message) {
                        runOnUiThread(new Runnable() {
                            public void run() {
                                setStatus("支付状态:" + message);
                            }
                        });
                    }
                });
            }
        });

        addButton(root, "退出确认(shouldQuitGame)", new Runnable() {
            public void run() {
                Operate.shouldQuitGame(MainActivity.this, new OnQuitGameListener() {
                    public void onQuit() {
                        runOnUiThread(new Runnable() {
                            public void run() {
                                setStatus("玩家确认退出");
                            }
                        });
                    }

                    public void onCancel() {
                        runOnUiThread(new Runnable() {
                            public void run() {
                                setStatus("取消退出");
                            }
                        });
                    }
                });
            }
        });

        addButton(root, "登出(logout)", new Runnable() {
            public void run() {
                Operate.logout();
            }
        });

        setContentView(scroll);
    }

    private void addButton(LinearLayout root, String text, final Runnable action) {
        Button b = new Button(this);
        b.setText(text);
        b.setAllCaps(false);
        LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(
                dp(320), LinearLayout.LayoutParams.WRAP_CONTENT);
        lp.topMargin = dp(10);
        b.setLayoutParams(lp);
        b.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                action.run();
            }
        });
        root.addView(b);
    }

    private void setStatus(String s) {
        status.setText(s);
    }

    private void toast(String s) {
        Toast.makeText(this, s, Toast.LENGTH_SHORT).show();
    }

    private int dp(int v) {
        return Math.round(v * getResources().getDisplayMetrics().density);
    }
}
