package com.m5755.sdk.ui;

import android.app.Activity;
import android.os.Handler;
import android.os.Looper;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.widget.Toast;

import com.m5755.operate.core.flow.ColdStartController;
import com.m5755.operate.core.flow.FlowUi;
import com.m5755.operate.core.gateway.Results;

import java.util.concurrent.Executor;

/**
 * 业务配套 UI:把冷启动状态机(sdk-core {@link ColdStartController})的 {@link FlowUi} 回调
 * 渲染为挂在宿主 Activity 上的模态层(07 §1.8 SDK 层挂载机制)。
 * 网关调用在后台线程,UI 回调统一 post 到主线程。
 */
public final class SdkUi implements FlowUi {

    private final Activity host;
    private final Executor background;
    private final Handler main = new Handler(Looper.getMainLooper());

    private ColdStartController controller;
    private FrameLayout overlay;

    // 验证码 60s 倒计时
    private TextView sendCodeButton;
    private int countdownLeft;
    private final Runnable countdownTick = new Runnable() {
        @Override
        public void run() {
            if (sendCodeButton == null) {
                return;
            }
            if (countdownLeft <= 0) {
                sendCodeButton.setEnabled(true);
                sendCodeButton.setText("重新发送");
                sendCodeButton.setTextColor(UiKit.PRIMARY_DEEP);
                return;
            }
            sendCodeButton.setText(countdownLeft + "s");
            sendCodeButton.setTextColor(UiKit.HINT);
            countdownLeft--;
            main.postDelayed(this, 1000);
        }
    };

    public SdkUi(Activity host, Executor background) {
        this.host = host;
        this.background = background;
    }

    public void setController(ColdStartController controller) {
        this.controller = controller;
    }

    // ===== FlowUi(均从后台线程调用 → post 到主线程) =====

    @Override
    public void showInitError(final String reason, final String message) {
        main.post(new Runnable() {
            public void run() {
                toast("初始化失败:" + (message == null ? reason : message));
            }
        });
    }

    @Override
    public void showMaintenance(final String message) {
        main.post(new Runnable() {
            public void run() {
                mountMaintenance(message);
            }
        });
    }

    @Override
    public void showProtocol(final String protocolVersion) {
        main.post(new Runnable() {
            public void run() {
                mountProtocol();
            }
        });
    }

    @Override
    public void showLoginWindow() {
        main.post(new Runnable() {
            public void run() {
                mountLogin();
            }
        });
    }

    @Override
    public void onSmsRequested(final Results.Sms result) {
        main.post(new Runnable() {
            public void run() {
                countdownLeft = 60;
                main.removeCallbacks(countdownTick);
                main.post(countdownTick);
                if (result != null && result.devCode != null) {
                    toast("调试验证码:" + result.devCode);
                } else {
                    toast("验证码已发送");
                }
            }
        });
    }

    @Override
    public void showLoginError(final String reason, final String message) {
        main.post(new Runnable() {
            public void run() {
                toast(loginErrorText(reason, message));
            }
        });
    }

    @Override
    public void onLoginSuccess(final Results.Login result) {
        // 登录链路诊断(08 §2.2):只输出 ID 类非密字段,不输出令牌。
        android.util.Log.i("M5755Sdk", "login_5755_account success platformAccountId="
                + result.platformAccountId + " isNewGameUser=" + result.isNewGameUser);
        main.post(new Runnable() {
            public void run() {
                dismiss();
                toast("5755 账户登录成功");
            }
        });
    }

    @Override
    public void onEntryBlockedByProtocolReject() {
        main.post(new Runnable() {
            public void run() {
                dismiss();
                toast("已退出登录流程");
            }
        });
    }

    // ===== 模态构建 =====

    private void mountProtocol() {
        LinearLayout card = UiKit.modalCard(host, 520);
        card.addView(UiKit.title(host, "个人信息保护引导"));
        TextView body = UiKit.hint(host,
                "本游戏接入 5755 SDK。为提供游戏资源加载、联网、账号安全、实名防沉迷、支付、用户中心和诊断能力,"
                        + "SDK 需要处理必要的设备信息、网络信息、当前游戏小号信息和日志信息。\n\n"
                        + "请阅读《用户注册协议》《用户隐私协议》《儿童隐私保护指引》《第三方信息共享清单》。同意后进入账号登录。");
        LinearLayout.LayoutParams bodyLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        bodyLp.topMargin = UiKit.dp(host, 14);
        card.addView(body, bodyLp);

        LinearLayout row = new LinearLayout(host);
        row.setOrientation(LinearLayout.HORIZONTAL);
        LinearLayout.LayoutParams rowLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 48));
        rowLp.topMargin = UiKit.dp(host, 18);
        TextView reject = UiKit.secondaryButton(host, "拒绝");
        TextView agree = UiKit.primaryButton(host, "同意");
        LinearLayout.LayoutParams half = new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1);
        half.rightMargin = UiKit.dp(host, 10);
        LinearLayout.LayoutParams half2 = new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1);
        half2.leftMargin = UiKit.dp(host, 10);
        // primaryButton 自带 topMargin,这里同排需清零
        agree.setLayoutParams(half2);
        reject.setLayoutParams(half);
        row.addView(reject);
        row.addView(agree);
        card.addView(row, rowLp);

        reject.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                background.execute(new Runnable() {
                    public void run() {
                        controller.onProtocolRejected();
                    }
                });
            }
        });
        agree.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                background.execute(new Runnable() {
                    public void run() {
                        controller.onProtocolConsented();
                    }
                });
            }
        });

        mount(card);
    }

    private void mountMaintenance(String message) {
        LinearLayout card = UiKit.modalCard(host, 420);
        card.addView(UiKit.title(host, "维护门禁"));
        String text = (message == null || message.isEmpty()) ? "当前游戏维护中,请稍后再试。" : message;
        LinearLayout.LayoutParams hintLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        hintLp.topMargin = UiKit.dp(host, 14);
        card.addView(UiKit.hint(host, text), hintLp);
        TextView ok = UiKit.primaryButton(host, "我知道了");
        card.addView(ok);
        ok.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                dismiss(); // 阻断进入,不触发账号变化
            }
        });
        mount(card);
    }

    private boolean protocolChecked = false;

    private void mountLogin() {
        protocolChecked = false;
        LinearLayout card = UiKit.modalCard(host, 340);

        // Tab 行(验证码登录 选中;密码登录 milestone 1 暂不开放)
        LinearLayout tabs = new LinearLayout(host);
        tabs.setOrientation(LinearLayout.HORIZONTAL);
        TextView tabSms = tab(host, "验证码登录", true);
        TextView tabPwd = tab(host, "密码登录", false);
        tabs.addView(tabSms);
        tabs.addView(tabPwd);
        card.addView(tabs);
        tabPwd.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                toast("密码登录暂未开放,请用验证码登录");
            }
        });

        TextView tip = new TextView(host);
        tip.setText("验证码用于登录,账号状态由平台识别");
        tip.setTextSize(12);
        tip.setTextColor(UiKit.MUTED);
        LinearLayout.LayoutParams tipLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        tipLp.topMargin = UiKit.dp(host, 14);
        card.addView(tip, tipLp);

        final android.widget.EditText phone = UiKit.input(host, "请输入手机号");
        phone.setInputType(android.text.InputType.TYPE_CLASS_PHONE);
        card.addView(phone);

        // 内嵌按钮输入行:验证码 + 发送验证码
        LinearLayout codeRow = new LinearLayout(host);
        codeRow.setOrientation(LinearLayout.HORIZONTAL);
        codeRow.setGravity(Gravity.CENTER_VERTICAL);
        codeRow.setBackground(UiKit.rounded(UiKit.WEAK, UiKit.dp(host, 6)));
        codeRow.setPadding(0, 0, UiKit.dp(host, 10), 0);
        LinearLayout.LayoutParams codeRowLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 44));
        codeRowLp.topMargin = UiKit.dp(host, 12);
        final android.widget.EditText code = new android.widget.EditText(host);
        code.setHint("请输入验证码");
        code.setSingleLine(true);
        code.setTextSize(14);
        code.setTextColor(UiKit.TEXT);
        code.setHintTextColor(UiKit.HINT);
        code.setBackgroundColor(0x00000000);
        code.setPadding(UiKit.dp(host, 14), 0, UiKit.dp(host, 8), 0);
        code.setInputType(android.text.InputType.TYPE_CLASS_NUMBER);
        codeRow.addView(code, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1));
        sendCodeButton = new TextView(host);
        sendCodeButton.setText("发送验证码");
        sendCodeButton.setTextSize(13);
        sendCodeButton.setTextColor(UiKit.PRIMARY_DEEP);
        sendCodeButton.setGravity(Gravity.CENTER);
        sendCodeButton.setClickable(true);
        codeRow.addView(sendCodeButton, new LinearLayout.LayoutParams(UiKit.dp(host, 104), ViewGroup.LayoutParams.MATCH_PARENT));
        card.addView(codeRow, codeRowLp);

        TextView loginBtn = UiKit.primaryButton(host, "登录");
        card.addView(loginBtn);

        // 协议勾选行
        LinearLayout checkRow = new LinearLayout(host);
        checkRow.setOrientation(LinearLayout.HORIZONTAL);
        checkRow.setGravity(Gravity.CENTER_VERTICAL);
        checkRow.setClickable(true);
        LinearLayout.LayoutParams checkLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        checkLp.topMargin = UiKit.dp(host, 12);
        final TextView box = new TextView(host);
        box.setWidth(UiKit.dp(host, 18));
        box.setHeight(UiKit.dp(host, 18));
        box.setGravity(Gravity.CENTER);
        box.setTextSize(12);
        box.setTextColor(UiKit.WHITE);
        box.setBackground(UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 9), 0xFFD5D7DD, UiKit.dp(host, 1)));
        TextView checkText = new TextView(host);
        checkText.setText("我已阅读并同意 用户协议 和 隐私政策");
        checkText.setTextSize(12);
        checkText.setTextColor(UiKit.CHECK_TEXT);
        LinearLayout.LayoutParams ctLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        ctLp.leftMargin = UiKit.dp(host, 8);
        checkRow.addView(box);
        checkRow.addView(checkText, ctLp);
        card.addView(checkRow, checkLp);
        checkRow.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                protocolChecked = !protocolChecked;
                if (protocolChecked) {
                    box.setText("✓");
                    box.setBackground(UiKit.rounded(UiKit.PRIMARY, UiKit.dp(host, 9)));
                } else {
                    box.setText("");
                    box.setBackground(UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 9), 0xFFD5D7DD, UiKit.dp(host, 1)));
                }
            }
        });

        sendCodeButton.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                final String p = phone.getText().toString().trim();
                if (p.isEmpty()) {
                    toast("请输入手机号");
                    return;
                }
                sendCodeButton.setEnabled(false);
                sendCodeButton.setText("发送中");
                sendCodeButton.setTextColor(UiKit.HINT);
                background.execute(new Runnable() {
                    public void run() {
                        controller.requestCode(p);
                    }
                });
            }
        });

        loginBtn.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                if (!protocolChecked) {
                    toast("请先阅读并同意协议");
                    return;
                }
                final String p = phone.getText().toString().trim();
                if (p.isEmpty()) {
                    toast("请输入手机号或账号");
                    return;
                }
                final String code1 = code.getText().toString().trim();
                if (code1.isEmpty()) {
                    toast("请输入验证码");
                    return;
                }
                background.execute(new Runnable() {
                    public void run() {
                        controller.submitLogin(p, code1);
                    }
                });
            }
        });

        mount(card);
    }

    private TextView tab(Activity c, String text, boolean selected) {
        TextView t = new TextView(c);
        t.setText(text);
        t.setTextSize(15);
        t.setGravity(Gravity.CENTER);
        t.getPaint().setFakeBoldText(selected);
        t.setTextColor(selected ? UiKit.TEXT : UiKit.TAB_UNSELECTED);
        t.setLayoutParams(new LinearLayout.LayoutParams(0, UiKit.dp(c, 46), 1));
        return t;
    }

    // ===== 挂载/卸载 =====

    private void mount(View card) {
        // 仅卸载旧层并取消倒计时;不清空 sendCodeButton(调用方刚为新层赋值)。
        main.removeCallbacks(countdownTick);
        detachOverlay();
        overlay = new FrameLayout(host);
        overlay.setBackgroundColor(UiKit.MASK);
        overlay.setClickable(true); // 拦截穿透,点击遮罩不关闭
        FrameLayout.LayoutParams cardLp = new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        cardLp.gravity = Gravity.CENTER;
        overlay.addView(card, cardLp);
        ViewGroup root = (ViewGroup) host.findViewById(android.R.id.content);
        root.addView(overlay, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
    }

    /** 卸载当前模态层并取消倒计时(完整收尾)。 */
    public void dismiss() {
        main.removeCallbacks(countdownTick);
        sendCodeButton = null;
        detachOverlay();
    }

    private void detachOverlay() {
        if (overlay != null) {
            ViewGroup parent = (ViewGroup) overlay.getParent();
            if (parent != null) {
                parent.removeView(overlay);
            }
            overlay = null;
        }
    }

    private void toast(String msg) {
        Toast.makeText(host, msg, Toast.LENGTH_SHORT).show();
    }

    private String loginErrorText(String reason, String message) {
        if (reason == null) {
            return message == null ? "登录失败" : message;
        }
        switch (reason) {
            case "sms_code_invalid":
                return "验证码错误,请重新输入";
            case "sms_code_expired":
                return "验证码已过期,请重新获取";
            case "sms_rate_limited":
                return "请求过于频繁,请稍后再试";
            case "maintenance":
                return "平台维护中,请稍后再试";
            case "param_invalid":
                return message == null ? "输入有误" : message;
            default:
                return message == null ? "登录失败" : message;
        }
    }
}
