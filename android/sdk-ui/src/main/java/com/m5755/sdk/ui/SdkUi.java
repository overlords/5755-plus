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

    // ===== 里程碑 2 界面(#16-#18) =====

    @Override
    public void showRealName() {
        main.post(new Runnable() {
            public void run() {
                mountRealName();
            }
        });
    }

    @Override
    public void showRealNameError(final String reason, final String message) {
        main.post(new Runnable() {
            public void run() {
                toast(message == null ? "实名信息格式有误" : message);
            }
        });
    }

    @Override
    public void showAntiAddictionBlocked(final String message) {
        main.post(new Runnable() {
            public void run() {
                mountSimpleNotice("防沉迷提示",
                        (message == null || message.isEmpty()) ? "当前账号受防沉迷限制,暂不能进入游戏。" : message);
            }
        });
    }

    @Override
    public void showSubaccountPicker(final Results.SubaccountList list, final String nickname, final boolean switchFlow) {
        main.post(new Runnable() {
            public void run() {
                mountPicker(list, nickname, switchFlow);
            }
        });
    }

    @Override
    public void showAutoEnterPrompt(final String account, final String displayName) {
        main.post(new Runnable() {
            public void run() {
                mountAutoEnter(account, displayName);
            }
        });
    }

    @Override
    public void showPickerNotice(final String message) {
        main.post(new Runnable() {
            public void run() {
                toast(message);
            }
        });
    }

    @Override
    public void showSessionCheck(final String account, final String maskedToken) {
        main.post(new Runnable() {
            public void run() {
                mountSessionCheck(account, maskedToken);
            }
        });
    }

    @Override
    public void showFlowBlocked(final String reason, final String message) {
        android.util.Log.i("M5755Sdk", "flow_blocked reason=" + reason);
        main.post(new Runnable() {
            public void run() {
                dismiss();
                toast(message == null ? "平台暂不可用,请稍后再试" : message);
            }
        });
    }

    private void mountRealName() {
        LinearLayout card = UiKit.modalCard(host, 420);
        card.addView(UiKit.title(host, "实名认证"));
        LinearLayout.LayoutParams hintLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        hintLp.topMargin = UiKit.dp(host, 14);
        card.addView(UiKit.hint(host, "根据相关规定,使用网络游戏须完成实名认证;信息仅用于合规校验。"), hintLp);
        final android.widget.EditText name = UiKit.input(host, "请输入真实姓名");
        card.addView(name);
        final android.widget.EditText idNo = UiKit.input(host, "请输入身份证号");
        card.addView(idNo);
        TextView submit = UiKit.primaryButton(host, "提交");
        card.addView(submit);
        submit.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                final String n = name.getText().toString().trim();
                final String id = idNo.getText().toString().trim();
                if (n.isEmpty()) {
                    toast("请输入真实姓名");
                    return;
                }
                if (id.isEmpty()) {
                    toast("请输入身份证号");
                    return;
                }
                background.execute(new Runnable() {
                    public void run() {
                        controller.submitRealName(n, id);
                    }
                });
            }
        });
        mount(card);
    }

    private void mountSimpleNotice(String title, String body) {
        LinearLayout card = UiKit.modalCard(host, 420);
        card.addView(UiKit.title(host, title));
        LinearLayout.LayoutParams hintLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        hintLp.topMargin = UiKit.dp(host, 14);
        card.addView(UiKit.hint(host, body), hintLp);
        TextView ok = UiKit.primaryButton(host, "我知道了");
        card.addView(ok);
        ok.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                dismiss();
            }
        });
        mount(card);
    }

    private void mountPicker(final Results.SubaccountList list, String nickname, final boolean switchFlow) {
        LinearLayout card = UiKit.modalCard(host, 560);
        // 头部:昵称 + 关闭
        LinearLayout header = new LinearLayout(host);
        header.setOrientation(LinearLayout.HORIZONTAL);
        header.setGravity(Gravity.CENTER_VERTICAL);
        TextView nick = new TextView(host);
        nick.setText(nickname == null || nickname.isEmpty() ? "5755玩家" : nickname);
        nick.setTextSize(17);
        nick.setTextColor(UiKit.TEXT);
        nick.getPaint().setFakeBoldText(true);
        header.addView(nick, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1));
        TextView close = new TextView(host);
        close.setText("×");
        close.setTextSize(22);
        close.setTextColor(0xFFA4A8B0);
        close.setGravity(Gravity.CENTER);
        close.setWidth(UiKit.dp(host, 42));
        close.setHeight(UiKit.dp(host, 42));
        close.setBackground(UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 21), 0xFFDEE1E8, UiKit.dp(host, 1)));
        header.addView(close);
        card.addView(header, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 52)));
        close.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                dismiss(); // 登录链路=进入未完成;切换链路=取消保持当前(03 §4.4)
                background.execute(new Runnable() {
                    public void run() {
                        controller.onPickerClosed(switchFlow);
                    }
                });
            }
        });

        // 标题行 + 添加小号
        LinearLayout titleRow = new LinearLayout(host);
        titleRow.setOrientation(LinearLayout.HORIZONTAL);
        titleRow.setGravity(Gravity.CENTER_VERTICAL);
        TextView st = new TextView(host);
        st.setText("选择小号进入游戏");
        st.setTextSize(15);
        st.setTextColor(UiKit.TEXT);
        st.getPaint().setFakeBoldText(true);
        titleRow.addView(st, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1));
        TextView add = new TextView(host);
        add.setText("添加小号");
        add.setTextSize(13);
        add.setTextColor(UiKit.PRIMARY_DEEP);
        add.getPaint().setFakeBoldText(true);
        add.setGravity(Gravity.CENTER);
        add.setPadding(UiKit.dp(host, 14), UiKit.dp(host, 6), UiKit.dp(host, 14), UiKit.dp(host, 6));
        add.setBackground(UiKit.roundedStroke(0x00000000, UiKit.dp(host, 8), UiKit.PRIMARY_DEEP, UiKit.dp(host, 1)));
        titleRow.addView(add);
        LinearLayout.LayoutParams trLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        trLp.topMargin = UiKit.dp(host, 10);
        card.addView(titleRow, trLp);
        add.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                background.execute(new Runnable() {
                    public void run() {
                        controller.onAddSubaccount(switchFlow);
                    }
                });
            }
        });

        // 列表(可滚动,300dp 上限)
        android.widget.ScrollView scroll = new android.widget.ScrollView(host);
        LinearLayout rows = new LinearLayout(host);
        rows.setOrientation(LinearLayout.VERTICAL);
        scroll.addView(rows);
        for (final Results.SubaccountList.Item it : list.items) {
            LinearLayout row = new LinearLayout(host);
            row.setOrientation(LinearLayout.HORIZONTAL);
            row.setGravity(Gravity.CENTER_VERTICAL);
            row.setBackground(UiKit.rounded(UiKit.WHITE, UiKit.dp(host, 8)));
            row.setPadding(UiKit.dp(host, 14), 0, UiKit.dp(host, 10), 0);
            TextView nameTv = new TextView(host);
            nameTv.setText(it.displayName);
            nameTv.setTextSize(14);
            nameTv.setTextColor(UiKit.TEXT);
            nameTv.getPaint().setFakeBoldText(true);
            row.addView(nameTv, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1));
            final TextView badge = new TextView(host);
            badge.setText(it.isDefault ? "✓ 默认" : "默认");
            badge.setTextSize(12);
            badge.setGravity(Gravity.CENTER);
            badge.setPadding(UiKit.dp(host, 10), UiKit.dp(host, 4), UiKit.dp(host, 10), UiKit.dp(host, 4));
            if (it.isDefault) {
                badge.setTextColor(UiKit.BTN_TEXT_ON_PRIMARY);
                badge.setBackground(UiKit.rounded(UiKit.PRIMARY, UiKit.dp(host, 999)));
            } else {
                badge.setTextColor(UiKit.MUTED);
                badge.setBackground(UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 999), 0xFFD5D7DD, UiKit.dp(host, 1)));
            }
            row.addView(badge);
            row.setClickable(true);
            row.setOnClickListener(new View.OnClickListener() {
                public void onClick(View v) {
                    background.execute(new Runnable() {
                        public void run() {
                            controller.onSubaccountChosen(it.account, switchFlow);
                        }
                    });
                }
            });
            badge.setOnClickListener(new View.OnClickListener() {
                public void onClick(View v) {
                    background.execute(new Runnable() {
                        public void run() {
                            controller.onSetDefault(it.account, switchFlow); // 点默认标签≠点行进入
                        }
                    });
                }
            });
            LinearLayout.LayoutParams rowLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 52));
            rowLp.topMargin = UiKit.dp(host, 8);
            rows.addView(row, rowLp);
        }
        LinearLayout.LayoutParams scLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        scroll.setBackground(UiKit.rounded(UiKit.WEAK, UiKit.dp(host, 10)));
        scroll.setPadding(UiKit.dp(host, 6), UiKit.dp(host, 2), UiKit.dp(host, 6), UiKit.dp(host, 8));
        scLp.topMargin = UiKit.dp(host, 10);
        int maxH = UiKit.dp(host, 300);
        scLp.height = Math.min(maxH, UiKit.dp(host, 60) * Math.max(1, list.items.size()));
        card.addView(scroll, scLp);

        TextView tip = new TextView(host);
        tip.setText("点击小号进入游戏,点击「默认」设为下次自动登录的小号。");
        tip.setTextSize(12);
        tip.setTextColor(UiKit.MUTED);
        LinearLayout.LayoutParams tipLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        tipLp.topMargin = UiKit.dp(host, 12);
        card.addView(tip, tipLp);

        mount(card);
    }

    private void mountAutoEnter(final String account, String displayName) {
        // 轻量提示条(07 §6):非模态、距顶 44dp、1800ms 后自动进入
        dismiss();
        final FrameLayout layer = new FrameLayout(host);
        layer.setClickable(false);
        LinearLayout bar = new LinearLayout(host);
        bar.setOrientation(LinearLayout.HORIZONTAL);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setBackground(UiKit.rounded(0xEEF7F8FA, UiKit.dp(host, 12)));
        bar.setElevation(UiKit.dp(host, 10));
        bar.setPadding(UiKit.dp(host, 18), 0, UiKit.dp(host, 10), 0);
        TextView text = new TextView(host);
        text.setText("将以「" + displayName + "」进入游戏");
        text.setTextSize(15);
        text.setTextColor(UiKit.TEXT);
        text.getPaint().setFakeBoldText(true);
        bar.addView(text, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1));
        TextView switchBtn = new TextView(host);
        switchBtn.setText("切换");
        switchBtn.setTextSize(14);
        switchBtn.setTextColor(UiKit.PRIMARY_DEEP);
        switchBtn.getPaint().setFakeBoldText(true);
        switchBtn.setPadding(UiKit.dp(host, 12), UiKit.dp(host, 8), UiKit.dp(host, 12), UiKit.dp(host, 8));
        bar.addView(switchBtn);
        FrameLayout.LayoutParams barLp = new FrameLayout.LayoutParams(
                UiKit.dp(host, 420), UiKit.dp(host, 60));
        barLp.gravity = Gravity.TOP | Gravity.CENTER_HORIZONTAL;
        barLp.topMargin = UiKit.dp(host, 44);
        layer.addView(bar, barLp);
        ViewGroup root = (ViewGroup) host.findViewById(android.R.id.content);
        root.addView(layer, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        overlay = layer;

        final Runnable proceed = new Runnable() {
            public void run() {
                dismiss();
                background.execute(new Runnable() {
                    public void run() {
                        controller.onAutoEnterElapsed(account);
                    }
                });
            }
        };
        main.postDelayed(proceed, 1800);
        switchBtn.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                main.removeCallbacks(proceed);
                dismiss();
                background.execute(new Runnable() {
                    public void run() {
                        controller.onAutoEnterSwitch();
                    }
                });
            }
        });
    }

    private void mountSessionCheck(String account, String maskedToken) {
        LinearLayout card = UiKit.modalCard(host, 420);
        card.addView(UiKit.title(host, "登录态校验"));
        TextView status = new TextView(host);
        status.setText("登录态校验通过");
        status.setTextSize(18);
        status.setTextColor(UiKit.TEXT);
        status.getPaint().setFakeBoldText(true);
        status.setGravity(Gravity.CENTER);
        status.setBackground(UiKit.rounded(0xFFFFF9DF, UiKit.dp(host, 8)));
        LinearLayout.LayoutParams stLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 52));
        stLp.topMargin = UiKit.dp(host, 14);
        card.addView(status, stLp);
        TextView detail = UiKit.hint(host, "当前游戏小号:" + account + "\n登录令牌:" + maskedToken
                + "\n游戏服务端将使用以上凭据完成登录态校验。");
        LinearLayout.LayoutParams dLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        dLp.topMargin = UiKit.dp(host, 14);
        card.addView(detail, dLp);
        TextView ok = UiKit.primaryButton(host, "进入游戏");
        card.addView(ok);
        ok.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                dismiss();
            }
        });
        mount(card);
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
