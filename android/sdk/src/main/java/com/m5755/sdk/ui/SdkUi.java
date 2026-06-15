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
    public void showAutoLoginPrompt(final String displayName) {
        main.post(new Runnable() {
            public void run() {
                toast(displayName == null || displayName.isEmpty()
                        ? "正在自动登录…" : "正在以 " + displayName + " 自动登录");
            }
        });
    }

    @Override
    public void showLoginError(final String reason, final String message) {
        main.post(new Runnable() {
            public void run() {
                resetSendCodeButton(); // 发送失败后复位,不卡"发送中"
                toast(loginErrorText(reason, message));
            }
        });
    }

    /** 复位验证码发送按钮(发送失败时不卡"发送中")。 */
    private void resetSendCodeButton() {
        if (sendCodeButton == null) {
            return;
        }
        main.removeCallbacks(countdownTick);
        sendCodeButton.setEnabled(true);
        sendCodeButton.setText("发送验证码");
        sendCodeButton.setTextColor(UiKit.PRIMARY_DEEP);
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
        // 固定高分段面板(还原生产 m5755):白底圆角 14;表头 64dp / 分隔线 / WEAK body(weight 填满)。
        android.util.DisplayMetrics dm = host.getResources().getDisplayMetrics();
        final int fixedH = Math.min(UiKit.dp(host, 430), Math.max(UiKit.dp(host, 320), dm.heightPixels - UiKit.dp(host, 70)));
        final int fixedW = Math.min(UiKit.dp(host, 480), dm.widthPixels - UiKit.dp(host, 40));

        LinearLayout card = new LinearLayout(host);
        card.setOrientation(LinearLayout.VERTICAL);
        card.setBackground(UiKit.rounded(UiKit.WHITE, UiKit.dp(host, 14)));
        card.setElevation(UiKit.dp(host, 18));
        card.setClipToOutline(true); // WEAK body 裁到圆角内

        // 表头:昵称 + ⇄ 切换5755账户(MUTED)
        LinearLayout header = new LinearLayout(host);
        header.setOrientation(LinearLayout.HORIZONTAL);
        header.setGravity(Gravity.CENTER_VERTICAL);
        header.setBackgroundColor(UiKit.WHITE);
        header.setPadding(UiKit.dp(host, 24), 0, UiKit.dp(host, 24), 0);
        TextView nick = new TextView(host);
        nick.setText(nickname == null || nickname.isEmpty() ? "5755玩家" : nickname);
        nick.setTextSize(17);
        nick.setTextColor(UiKit.TEXT);
        nick.getPaint().setFakeBoldText(true);
        nick.setSingleLine(true);
        nick.setEllipsize(android.text.TextUtils.TruncateAt.END);
        header.addView(nick, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1));
        TextView switchAcc = new TextView(host);
        switchAcc.setText("⇄");
        switchAcc.setTextSize(22);
        switchAcc.setTextColor(UiKit.MUTED);
        switchAcc.setGravity(Gravity.CENTER);
        switchAcc.setContentDescription("切换5755账户");
        header.addView(switchAcc, new LinearLayout.LayoutParams(UiKit.dp(host, 42), UiKit.dp(host, 42)));
        card.addView(header, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 64)));
        switchAcc.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                background.execute(new Runnable() {
                    public void run() {
                        controller.logout(); // ⇄=切换5755账户:清理并回登录窗(03 §6)
                    }
                });
            }
        });

        // 分隔线
        View divider = new View(host);
        divider.setBackgroundColor(UiKit.LINE);
        card.addView(divider, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 1));

        // body(weight 1,WEAK 底)
        LinearLayout body = new LinearLayout(host);
        body.setOrientation(LinearLayout.VERTICAL);
        body.setBackgroundColor(UiKit.WEAK);
        body.setPadding(UiKit.dp(host, 24), UiKit.dp(host, 12), UiKit.dp(host, 24), UiKit.dp(host, 8));
        card.addView(body, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1));

        // 标题行:选择小号进入游戏 + ! 信息标 + 添加小号
        LinearLayout titleRow = new LinearLayout(host);
        titleRow.setOrientation(LinearLayout.HORIZONTAL);
        titleRow.setGravity(Gravity.CENTER_VERTICAL);
        LinearLayout titleText = new LinearLayout(host);
        titleText.setOrientation(LinearLayout.HORIZONTAL);
        titleText.setGravity(Gravity.CENTER_VERTICAL);
        TextView st = new TextView(host);
        st.setText("选择小号进入游戏");
        st.setTextSize(16);
        st.setTextColor(UiKit.TEXT);
        st.getPaint().setFakeBoldText(true);
        titleText.addView(st);
        TextView info = new TextView(host);
        info.setText("!");
        info.setTextSize(11);
        info.setTextColor(UiKit.MUTED);
        info.getPaint().setFakeBoldText(true);
        info.setGravity(Gravity.CENTER);
        info.setIncludeFontPadding(false);
        info.setBackground(UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 999), UiKit.LINE, UiKit.dp(host, 1)));
        LinearLayout.LayoutParams infoLp = new LinearLayout.LayoutParams(UiKit.dp(host, 18), UiKit.dp(host, 18));
        infoLp.leftMargin = UiKit.dp(host, 8);
        titleText.addView(info, infoLp);
        titleRow.addView(titleText, new LinearLayout.LayoutParams(0, UiKit.dp(host, 36), 1));
        info.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                toast("游戏小号是你在本游戏内的角色账号,由平台分配;点「默认」可设为下次自动登录的小号。");
            }
        });

        final boolean full = list.items.size() >= 10;
        TextView add = new TextView(host);
        add.setText("添加小号");
        add.setTextSize(13);
        add.getPaint().setFakeBoldText(true);
        add.setGravity(Gravity.CENTER);
        if (full) {
            add.setTextColor(0xFFA6A9B0);
            add.setBackground(UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 8), UiKit.LINE, UiKit.dp(host, 1)));
        } else {
            add.setTextColor(UiKit.PRIMARY_DEEP);
            add.setBackground(UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 8), UiKit.PRIMARY_DEEP, UiKit.dp(host, 1)));
        }
        titleRow.addView(add, new LinearLayout.LayoutParams(UiKit.dp(host, 86), UiKit.dp(host, 32)));
        body.addView(titleRow, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 36)));
        add.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                if (full) {
                    toast("最多添加10个小号哦");
                    return;
                }
                background.execute(new Runnable() {
                    public void run() {
                        controller.onAddSubaccount(switchFlow);
                    }
                });
            }
        });

        // 列表(weight 1 填满 body 余高;>3 时右侧 3dp 金色滚动条装饰)
        FrameLayout listFrame = new FrameLayout(host);
        android.widget.ScrollView scroll = new android.widget.ScrollView(host);
        scroll.setVerticalScrollBarEnabled(false);
        scroll.setClipToPadding(false);
        scroll.setPadding(0, 0, UiKit.dp(host, 16), UiKit.dp(host, 8));
        LinearLayout rows = new LinearLayout(host);
        rows.setOrientation(LinearLayout.VERTICAL);
        scroll.addView(rows);
        listFrame.addView(scroll, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        if (list.items.size() > 3) {
            // 金色滚动条:高度按可见/总高比例、translationY 跟随 scrollY 实时移动(非静态装饰)
            final View thumb = new View(host);
            thumb.setBackground(UiKit.rounded(UiKit.PRIMARY, UiKit.dp(host, 999)));
            final FrameLayout.LayoutParams thumbLp = new FrameLayout.LayoutParams(UiKit.dp(host, 3), UiKit.dp(host, 40), Gravity.END | Gravity.TOP);
            thumbLp.rightMargin = UiKit.dp(host, 3);
            thumb.setVisibility(View.GONE);
            listFrame.addView(thumb, thumbLp);
            final android.widget.ScrollView fscroll = scroll;
            final LinearLayout frows = rows;
            final int minThumb = UiKit.dp(host, 28);
            final Runnable upd = new Runnable() {
                public void run() {
                    int totalH = frows.getHeight() + fscroll.getPaddingTop() + fscroll.getPaddingBottom();
                    int trackH = fscroll.getHeight();
                    if (totalH <= trackH || trackH <= 0) {
                        thumb.setVisibility(View.GONE);
                        return;
                    }
                    thumb.setVisibility(View.VISIBLE);
                    int th = Math.max(minThumb, (int) ((long) trackH * trackH / totalH));
                    if (thumbLp.height != th) {
                        thumbLp.height = th;
                        thumb.setLayoutParams(thumbLp);
                    }
                    int maxScroll = totalH - trackH;
                    int maxY = trackH - th;
                    float ty = Math.max(0, Math.min(maxY, (float) fscroll.getScrollY() / maxScroll * maxY));
                    thumb.setTranslationY(ty);
                }
            };
            scroll.getViewTreeObserver().addOnScrollChangedListener(new android.view.ViewTreeObserver.OnScrollChangedListener() {
                public void onScrollChanged() {
                    upd.run();
                }
            });
            scroll.post(upd);
        }
        body.addView(listFrame, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1));
        // 小号行(还原 m5755 smallAccountItem,卡片高调至 58dp 求长宽比协调)——白卡/3dp 圆角/LINE 细边/elevation 2;
        // 名 14sp 粗;右侧 20dp 金圆 + chevron 矢量图(tint #5D4300);左上角「默认」徽标(圆选 + 标签,骑卡片顶边)。
        int rowIdx = 0;
        for (final Results.SubaccountList.Item it : list.items) {
            FrameLayout wrap = new FrameLayout(host);
            wrap.setClickable(true);

            FrameLayout item = new FrameLayout(host);
            item.setBackground(UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 3), UiKit.LINE, UiKit.dp(host, 1)));
            item.setElevation(UiKit.dp(host, 2));
            FrameLayout.LayoutParams itemLp = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 58), Gravity.TOP);
            itemLp.topMargin = UiKit.dp(host, 14);
            wrap.addView(item, itemLp);

            TextView nameTv = new TextView(host);
            nameTv.setText(it.displayName);
            nameTv.setTextSize(14);
            nameTv.setTextColor(UiKit.TEXT);
            nameTv.getPaint().setFakeBoldText(true);
            nameTv.setIncludeFontPadding(false);
            nameTv.setGravity(Gravity.CENTER_VERTICAL);
            FrameLayout.LayoutParams nameLp = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT);
            nameLp.leftMargin = UiKit.dp(host, 16);
            nameLp.rightMargin = UiKit.dp(host, 56);
            item.addView(nameTv, nameLp);

            android.widget.ImageView enter = new android.widget.ImageView(host);
            enter.setContentDescription("进入");
            enter.setBackground(UiKit.rounded(UiKit.PRIMARY, UiKit.dp(host, 999)));
            enter.setImageResource(com.m5755.operate.R.drawable.m5755_ic_chevron_right_24);
            enter.setColorFilter(UiKit.BTN_TEXT_ON_PRIMARY);
            enter.setScaleType(android.widget.ImageView.ScaleType.CENTER);
            enter.setPadding(UiKit.dp(host, 3), UiKit.dp(host, 3), UiKit.dp(host, 3), UiKit.dp(host, 3));
            FrameLayout.LayoutParams enLp = new FrameLayout.LayoutParams(UiKit.dp(host, 20), UiKit.dp(host, 20), Gravity.END | Gravity.CENTER_VERTICAL);
            enLp.rightMargin = UiKit.dp(host, 8);
            item.addView(enter, enLp);

            // 「默认」徽标:6dp 药丸 + 14dp 圆选 + 标签(圆选与文字垂直居中对齐),骑卡片顶边左上角(左2/上4)
            final LinearLayout badge = new LinearLayout(host);
            badge.setOrientation(LinearLayout.HORIZONTAL);
            badge.setGravity(Gravity.CENTER_VERTICAL);
            badge.setPadding(UiKit.dp(host, 6), 0, UiKit.dp(host, 7), 0);
            badge.setBackground(UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 6), UiKit.LINE, UiKit.dp(host, 1)));
            badge.setElevation(UiKit.dp(host, 4));
            TextView radio = new TextView(host);
            radio.setText(it.isDefault ? "✓" : "");
            radio.setTextSize(10);
            radio.getPaint().setFakeBoldText(true);
            radio.setIncludeFontPadding(false);
            radio.setGravity(Gravity.CENTER);
            radio.setTextColor(UiKit.BTN_TEXT_ON_PRIMARY);
            radio.setBackground(it.isDefault
                    ? UiKit.rounded(UiKit.PRIMARY, UiKit.dp(host, 999))
                    : UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 999), UiKit.LINE, UiKit.dp(host, 1)));
            LinearLayout.LayoutParams radLp = new LinearLayout.LayoutParams(UiKit.dp(host, 14), UiKit.dp(host, 14));
            radLp.rightMargin = UiKit.dp(host, 4);
            badge.addView(radio, radLp);
            TextView dtext = new TextView(host);
            dtext.setText("默认");
            dtext.setTextSize(10);
            dtext.setTextColor(UiKit.MUTED);
            dtext.setIncludeFontPadding(false);
            dtext.setGravity(Gravity.CENTER_VERTICAL);
            badge.addView(dtext, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT));
            FrameLayout.LayoutParams badgeLp = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, UiKit.dp(host, 22), Gravity.START | Gravity.TOP);
            badgeLp.leftMargin = UiKit.dp(host, 2);
            badgeLp.topMargin = UiKit.dp(host, 4);
            wrap.addView(badge, badgeLp);

            wrap.setOnClickListener(new View.OnClickListener() {
                public void onClick(View v) {
                    background.execute(new Runnable() {
                        public void run() {
                            controller.onSubaccountChosen(it.account, switchFlow);
                        }
                    });
                }
            });
            badge.setClickable(true);
            badge.setOnClickListener(new View.OnClickListener() {
                public void onClick(View v) {
                    background.execute(new Runnable() {
                        public void run() {
                            controller.onSetDefault(it.account, switchFlow); // 点默认徽标≠点行进入
                        }
                    });
                }
            });
            LinearLayout.LayoutParams wrapLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 72));
            wrapLp.topMargin = UiKit.dp(host, rowIdx == 0 ? 12 : 6);
            rowIdx++;
            rows.addView(wrap, wrapLp);
        }
        mount(card);
        // 固定面板尺寸(mount 默认 WRAP_CONTENT;此处压成定宽高,让 body weight 撑开)
        card.getLayoutParams().width = fixedW;
        card.getLayoutParams().height = fixedH;
        card.requestLayout();

        // 骑角关闭 ×(还原 m5755):加在 overlay 上、中心对准面板右上角(card 与 × 同为 CENTER,
        // 再 translation 半个面板宽高即落到角上,避开状态栏坐标换算),点击关闭选择页。
        TextView closeX = new TextView(host);
        closeX.setText("×");
        closeX.setTextSize(22);
        closeX.setTextColor(0xFFA4A8B0);
        closeX.setGravity(Gravity.CENTER);
        closeX.setIncludeFontPadding(false);
        closeX.setBackground(UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 21), 0xFFDEE1E8, UiKit.dp(host, 1)));
        closeX.setElevation(UiKit.dp(host, 22));
        closeX.setContentDescription("关闭小号选择页");
        int xSize = UiKit.dp(host, 42);
        overlay.addView(closeX, new FrameLayout.LayoutParams(xSize, xSize, Gravity.CENTER));
        closeX.setTranslationX(fixedW / 2f);
        closeX.setTranslationY(-fixedH / 2f);
        closeX.bringToFront();
        closeX.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                dismiss();
                background.execute(new Runnable() {
                    public void run() {
                        controller.onPickerClosed(switchFlow);
                    }
                });
            }
        });
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

    // ===== 里程碑 3 界面(#26-#28) =====

    @Override
    public void showRoleResult(final boolean success, final String reason, final java.util.Map<String, String> fields) {
        main.post(new Runnable() {
            public void run() {
                mountRoleResult(success, fields);
            }
        });
    }

    @Override
    public void showPayDrawer(final java.util.Map<String, String> display, final String paymentUrl) {
        main.post(new Runnable() {
            public void run() {
                mountPayDrawer(display, paymentUrl);
            }
        });
    }

    // #5:用户中心 = 平台 H5;URL 经 /config 下发,加载时带 platformToken。
    private String userCenterUrl = "";
    private String platformToken = "";

    @Override
    public void showFloatBall(final String account, final String ucUrl, final String token) {
        main.post(new Runnable() {
            public void run() {
                userCenterUrl = ucUrl == null ? "" : ucUrl;
                platformToken = token == null ? "" : token;
                mountFloatBall(account);
            }
        });
    }

    @Override
    public void hideFloatBall() {
        main.post(new Runnable() {
            public void run() {
                if (floatBall != null) {
                    ViewGroup p = (ViewGroup) floatBall.getParent();
                    if (p != null) {
                        p.removeView(floatBall);
                    }
                    floatBall = null;
                }
                dismiss();
            }
        });
    }

    private View floatBall;
    private String floatAccount;

    private void mountRoleResult(boolean success, java.util.Map<String, String> fields) {
        LinearLayout card = UiKit.modalCard(host, 420);
        card.addView(UiKit.title(host, "角色上报"));
        TextView status = new TextView(host);
        status.setText(success ? "角色上报成功" : "角色上报失败");
        status.setTextSize(18);
        status.setTextColor(UiKit.TEXT);
        status.getPaint().setFakeBoldText(true);
        status.setGravity(Gravity.CENTER);
        status.setBackground(UiKit.rounded(0xFFFFF9DF, UiKit.dp(host, 8)));
        LinearLayout.LayoutParams stLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 52));
        stLp.topMargin = UiKit.dp(host, 14);
        card.addView(status, stLp);
        StringBuilder sb = new StringBuilder();
        sb.append("区服:").append(readable(fields.get("serverName"))).append("\n");
        sb.append("角色:").append(readable(fields.get("roleName"))).append("\n");
        sb.append("等级:").append(readable(fields.get("roleLevel"))).append("\n");
        sb.append("战力:").append(readable(fields.get("roleCe"))).append("\n");
        sb.append("累计充值:").append(readable(fields.get("roleRechargeAmount")));
        LinearLayout.LayoutParams dLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        dLp.topMargin = UiKit.dp(host, 14);
        card.addView(UiKit.hint(host, sb.toString()), dLp);
        TextView ok = UiKit.primaryButton(host, "我知道了");
        card.addView(ok);
        ok.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                dismiss();
            }
        });
        mount(card);
    }

    /** -1 占位渲染为"—"(07 §0.3)。 */
    private static String readable(String v) {
        if (v == null || v.isEmpty() || "-1".equals(v)) {
            return "—";
        }
        return v;
    }

    private void mountPayDrawer(java.util.Map<String, String> display, String paymentUrl) {
        boolean portrait = host.getResources().getConfiguration().orientation
                == android.content.res.Configuration.ORIENTATION_PORTRAIT;
        dismiss();
        overlay = new FrameLayout(host);
        overlay.setBackgroundColor(UiKit.MASK);
        overlay.setClickable(true);

        LinearLayout panel = new LinearLayout(host);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setBackgroundColor(0xFFF5F5F5);
        panel.setPadding(UiKit.dp(host, 14), UiKit.dp(host, 10), UiKit.dp(host, 14), UiKit.dp(host, 14));

        FrameLayout.LayoutParams plp;
        int dm = host.getResources().getDisplayMetrics().widthPixels;
        int dmh = host.getResources().getDisplayMetrics().heightPixels;
        if (portrait) {
            // 竖屏底部抽屉(07 §1.12):≤80% 高、顶圆角、抓手条
            panel.setBackground(topRounded(0xFFF5F5F5, UiKit.dp(host, 16)));
            TextView grab = new TextView(host);
            grab.setBackground(UiKit.rounded(0xFFCFD2D8, UiKit.dp(host, 999)));
            LinearLayout.LayoutParams glp = new LinearLayout.LayoutParams(UiKit.dp(host, 40), UiKit.dp(host, 4));
            glp.gravity = Gravity.CENTER_HORIZONTAL;
            glp.bottomMargin = UiKit.dp(host, 6);
            panel.addView(grab, glp);
            plp = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, (int) (dmh * 0.8));
            plp.gravity = Gravity.BOTTOM;
        } else {
            // 横屏右侧全高抽屉
            int w = Math.min(dm, Math.max(UiKit.dp(host, 520), dm / 2));
            plp = new FrameLayout.LayoutParams(w, ViewGroup.LayoutParams.MATCH_PARENT);
            plp.gravity = Gravity.END;
        }

        // 头部
        LinearLayout header = new LinearLayout(host);
        header.setOrientation(LinearLayout.HORIZONTAL);
        header.setGravity(Gravity.CENTER_VERTICAL);
        TextView back = new TextView(host);
        back.setText(portrait ? "⌄" : "‹");
        back.setTextSize(26);
        back.setTextColor(UiKit.PRIMARY_DEEP);
        back.setWidth(UiKit.dp(host, 44));
        back.setGravity(Gravity.CENTER);
        header.addView(back);
        TextView title = new TextView(host);
        title.setText("5755 游戏支付");
        title.setTextSize(20);
        title.setTextColor(0xFF111111);
        title.setGravity(Gravity.CENTER);
        header.addView(title, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1));
        header.addView(new TextView(host), new LinearLayout.LayoutParams(UiKit.dp(host, 44), 1));
        panel.addView(header, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 48)));
        back.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                dismiss();
                restoreFloatBall();
                notifyPayClosed(false); // 付款前关闭订单确认抽屉=未完成(05 §3.1)
            }
        });

        // 订单卡(逐字段取自入参)
        android.widget.ScrollView scroll = new android.widget.ScrollView(host);
        LinearLayout card = new LinearLayout(host);
        card.setOrientation(LinearLayout.VERTICAL);
        card.setBackground(UiKit.rounded(UiKit.WHITE, UiKit.dp(host, 14)));
        card.setPadding(UiKit.dp(host, 20), UiKit.dp(host, 4), UiKit.dp(host, 20), UiKit.dp(host, 4));
        for (java.util.Map.Entry<String, String> e : display.entrySet()) {
            LinearLayout row = new LinearLayout(host);
            row.setOrientation(LinearLayout.HORIZONTAL);
            row.setGravity(Gravity.CENTER_VERTICAL);
            TextView k = new TextView(host);
            k.setText(e.getKey());
            k.setTextSize(16);
            k.setTextColor(0xFF222222);
            k.setWidth(UiKit.dp(host, 76));
            TextView val = new TextView(host);
            val.setText(readable(e.getValue()));
            val.setTextSize(15);
            val.setTextColor(0xFF6C6C6C);
            row.addView(k);
            row.addView(val, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1));
            card.addView(row, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 44)));
        }
        scroll.addView(card);
        LinearLayout.LayoutParams scLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1);
        scLp.topMargin = UiKit.dp(host, 12);
        panel.addView(scroll, scLp);

        // 支付说明(发放以充值回调为准——固化口径)
        TextView explain = UiKit.hint(host, "当前页面只承载 SDK 自有支付流程。游戏内物品发放以游戏服务端收到并校验通过的充值回调为准,客户端通知只用于界面状态。");
        LinearLayout.LayoutParams exLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        exLp.topMargin = UiKit.dp(host, 10);
        panel.addView(explain, exLp);

        // 底部支付栏
        LinearLayout payBar = new LinearLayout(host);
        payBar.setOrientation(LinearLayout.HORIZONTAL);
        payBar.setGravity(Gravity.CENTER_VERTICAL);
        payBar.setBackground(UiKit.rounded(0xFF3F3F3F, UiKit.dp(host, 999)));
        TextView amount = new TextView(host);
        amount.setText("应付:" + display.get("金额"));
        amount.setTextSize(20);
        amount.setTextColor(UiKit.WHITE);
        amount.setGravity(Gravity.CENTER);
        payBar.addView(amount, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1));
        TextView confirm = new TextView(host);
        confirm.setText("确认支付");
        confirm.setTextSize(20);
        confirm.setTextColor(UiKit.WHITE);
        confirm.getPaint().setFakeBoldText(true);
        confirm.setGravity(Gravity.CENTER);
        confirm.setBackgroundColor(0xFFFF4962);
        payBar.addView(confirm, new LinearLayout.LayoutParams(UiKit.dp(host, 156), ViewGroup.LayoutParams.MATCH_PARENT));
        LinearLayout.LayoutParams pbLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 58));
        pbLp.topMargin = UiKit.dp(host, 12);
        panel.addView(payBar, pbLp);
        confirm.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                if (paymentUrl != null && !paymentUrl.isEmpty()) {
                    // 生产:交接到平台收银台(07 §9)——容器内远程 WebView 加载,渠道 App 外跳待 #61/§5
                    mountCashier(paymentUrl);
                } else {
                    // 演示/未接线:无收银台入口,提示后按未完成收口(不伪造支付结果、不假报已交接)
                    toast("支付处理中,等待服务端充值回调");
                    dismiss();
                    restoreFloatBall();
                    notifyPayClosed(false);
                }
            }
        });

        overlay.addView(panel, plp);
        ViewGroup root = (ViewGroup) host.findViewById(android.R.id.content);
        root.addView(overlay, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
    }

    /**
     * 平台收银台(生产支付,07 §9):订单确认后交接,在 SDK 自有支付容器内以远程 WebView 加载平台
     * 收银台 H5(下单返回的 paymentUrl),套 §1.13 品牌加载态(占位 / 就绪淡入 / 失败重试)。玩家在
     * 收银台内选择支付方式并付款。loadableWeb 传 allowPaySchemes=true:http(s) 站内加载,微信/支付宝
     * 白名单 scheme 经 startActivity(VIEW) 外跳拉起渠道 App(支付域受限外跳例外,已评审通过,ADR-0014 /
     * 01 §4.2;未安装泛化兜底、零 queries)。原生层只出现「5755 游戏支付」,不含 07 §0.2 禁词。
     */
    @SuppressWarnings("SetJavaScriptEnabled")
    private void mountCashier(final String paymentUrl) {
        boolean portrait = host.getResources().getConfiguration().orientation
                == android.content.res.Configuration.ORIENTATION_PORTRAIT;
        dismiss();
        overlay = new FrameLayout(host);
        overlay.setBackgroundColor(UiKit.MASK);
        overlay.setClickable(true);

        LinearLayout panel = new LinearLayout(host);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setPadding(UiKit.dp(host, 14), UiKit.dp(host, 10), UiKit.dp(host, 14), UiKit.dp(host, 14));

        FrameLayout.LayoutParams plp;
        int dm = host.getResources().getDisplayMetrics().widthPixels;
        int dmh = host.getResources().getDisplayMetrics().heightPixels;
        if (portrait) {
            // 竖屏底部抽屉(07 §1.12):≤80% 高、顶圆角、抓手条(与订单确认抽屉同形态)
            panel.setBackground(topRounded(0xFFF5F5F5, UiKit.dp(host, 16)));
            TextView grab = new TextView(host);
            grab.setBackground(UiKit.rounded(0xFFCFD2D8, UiKit.dp(host, 999)));
            LinearLayout.LayoutParams glp = new LinearLayout.LayoutParams(UiKit.dp(host, 40), UiKit.dp(host, 4));
            glp.gravity = Gravity.CENTER_HORIZONTAL;
            glp.bottomMargin = UiKit.dp(host, 6);
            panel.addView(grab, glp);
            plp = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, (int) (dmh * 0.8));
            plp.gravity = Gravity.BOTTOM;
        } else {
            // 横屏右侧全高抽屉
            panel.setBackgroundColor(0xFFF5F5F5);
            int w = Math.min(dm, Math.max(UiKit.dp(host, 520), dm / 2));
            plp = new FrameLayout.LayoutParams(w, ViewGroup.LayoutParams.MATCH_PARENT);
            plp.gravity = Gravity.END;
        }

        // 头部:返回/收起 + 标题(原生层不出现 §0.2 禁词)
        LinearLayout header = new LinearLayout(host);
        header.setOrientation(LinearLayout.HORIZONTAL);
        header.setGravity(Gravity.CENTER_VERTICAL);
        final TextView back = new TextView(host);
        back.setText(portrait ? "⌄" : "‹");
        back.setTextSize(26);
        back.setTextColor(UiKit.PRIMARY_DEEP);
        back.setWidth(UiKit.dp(host, 44));
        back.setGravity(Gravity.CENTER);
        header.addView(back);
        TextView title = new TextView(host);
        title.setText("5755 游戏支付");
        title.setTextSize(20);
        title.setTextColor(0xFF111111);
        title.setGravity(Gravity.CENTER);
        header.addView(title, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1));
        header.addView(new TextView(host), new LinearLayout.LayoutParams(UiKit.dp(host, 44), 1));
        panel.addView(header, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 48)));

        // 收银台远程 WebView(平台 H5;§1.13 加载态;http(s) 站内、微信/支付宝白名单 scheme 外跳——ADR-0014)
        final android.webkit.WebView web = new android.webkit.WebView(host);
        android.webkit.WebSettings ws = web.getSettings();
        ws.setJavaScriptEnabled(true);
        ws.setJavaScriptCanOpenWindowsAutomatically(false);
        ws.setAllowFileAccess(false);
        ws.setAllowFileAccessFromFileURLs(false);
        ws.setAllowUniversalAccessFromFileURLs(false);
        ws.setUserAgentString(ws.getUserAgentString() + " M5755Sdk/" + SDK_VERSION_UA);
        LinearLayout.LayoutParams webLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, 0, 1);
        webLp.topMargin = UiKit.dp(host, 10);
        panel.addView(loadableWeb(web, paymentUrl, true), webLp); // 收银台:支付域受限外跳例外(ADR-0014)

        back.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                web.loadUrl("about:blank");
                dismiss();
                restoreFloatBall();
                notifyPayClosed(false); // 收银台关闭无结果信号→保守判未完成(05 §3.1;sentinel 区分待 #60)
            }
        });

        overlay.addView(panel, plp);
        ViewGroup root = (ViewGroup) host.findViewById(android.R.id.content);
        root.addView(overlay, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
    }

    private void mountFloatBall(String account) {
        this.floatAccount = account;
        if (floatBall != null) {
            return; // 已存在
        }
        final TextView ball = new TextView(host);
        ball.setText("账");
        ball.setTextSize(15);
        ball.setTextColor(UiKit.WHITE);
        ball.getPaint().setFakeBoldText(true);
        ball.setGravity(Gravity.CENTER);
        ball.setBackground(UiKit.rounded(0xD62A303E, UiKit.dp(host, 999)));
        int size = UiKit.dp(host, 48);
        FrameLayout.LayoutParams lp = new FrameLayout.LayoutParams(size, size);
        lp.gravity = Gravity.START | Gravity.CENTER_VERTICAL;
        lp.leftMargin = UiKit.dp(host, 8);
        ball.setLayoutParams(lp);
        ball.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                openUserCenter(floatAccount);
            }
        });
        ViewGroup root = (ViewGroup) host.findViewById(android.R.id.content);
        root.addView(ball);
        floatBall = ball;
    }

    private void restoreFloatBall() {
        if (floatBall == null && floatAccount != null) {
            mountFloatBall(floatAccount);
        }
    }

    /**
     * 支付容器终态信号 → controller.onPayContainerClosed(背景单线程,与 recharge 同执行器,
     * 保证客户端支付回调单次)。照 onUserCenterAction 的桥模式;handed 区分已交接/未完成。
     */
    private void notifyPayClosed(final boolean handed) {
        if (controller == null) {
            return;
        }
        background.execute(new Runnable() {
            public void run() {
                controller.onPayContainerClosed(handed);
            }
        });
    }

    @SuppressWarnings({"SetJavaScriptEnabled", "AddJavascriptInterface"})
    private void openUserCenter(final String account) {
        dismiss();
        overlay = new FrameLayout(host);
        overlay.setBackgroundColor(UiKit.MASK);
        overlay.setClickable(true);

        final android.webkit.WebView web = new android.webkit.WebView(host);
        android.webkit.WebSettings ws = web.getSettings();
        ws.setJavaScriptEnabled(true);
        ws.setJavaScriptCanOpenWindowsAutomatically(false);
        ws.setAllowFileAccess(false);
        ws.setAllowFileAccessFromFileURLs(false);
        ws.setAllowUniversalAccessFromFileURLs(false);
        ws.setUserAgentString(ws.getUserAgentString() + " M5755Sdk/" + SDK_VERSION_UA); // UA 带 SDK 版本号
        web.addJavascriptInterface(new UserCenterBridge(), "UserCenter");
        final View centerView;
        if (userCenterUrl != null && !userCenterUrl.isEmpty()) {
            // #5:平台用户中心 H5,带 platformToken;§1.13 套加载态(占位 + 就绪淡入 + 失败重试)
            String sep = userCenterUrl.contains("?") ? "&" : "?";
            centerView = loadableWeb(web, userCenterUrl + sep + "token=" + android.net.Uri.encode(platformToken), false);
        } else {
            // 未配置 URL:瞬时本地回退页,不套加载态(避免一闪)
            web.setWebViewClient(new android.webkit.WebViewClient() {
                @Override
                public boolean shouldOverrideUrlLoading(android.webkit.WebView v, String u) {
                    if (u != null && (u.startsWith("http://") || u.startsWith("https://"))) {
                        v.loadUrl(u); // 站内加载,不外跳系统浏览器
                    }
                    return true;
                }
            });
            web.loadDataWithBaseURL(null, userCenterFallbackHtml(), "text/html", "utf-8", null); // 未配置 URL 的最小回退
            centerView = web;
        }

        int dm = host.getResources().getDisplayMetrics().widthPixels;
        boolean portrait = host.getResources().getConfiguration().orientation
                == android.content.res.Configuration.ORIENTATION_PORTRAIT;
        int w = Math.min(dm, Math.max(UiKit.dp(host, 520), (int) (dm * 0.58)));
        if (portrait) {
            w = Math.min(w, (int) (dm * 0.8)); // 竖屏 ≤80% 留游戏可见条
        }
        FrameLayout.LayoutParams wlp = new FrameLayout.LayoutParams(w, ViewGroup.LayoutParams.MATCH_PARENT);
        wlp.gravity = Gravity.START;
        overlay.addView(centerView, wlp);
        ViewGroup root = (ViewGroup) host.findViewById(android.R.id.content);
        root.addView(overlay, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
    }

    /** JS Bridge 最小契约(06 §3,#5):仅 postAccountAction;不再下发账户上下文。 */
    private final class UserCenterBridge {

        @android.webkit.JavascriptInterface
        public void postAccountAction(String action) {
            final String a = ("logout".equals(action) || "switch_account".equals(action) || "session_invalid".equals(action))
                    ? action : "unknown";
            main.post(new Runnable() {
                public void run() {
                    dismiss();
                    if (!"unknown".equals(a)) {
                        background.execute(new Runnable() {
                            public void run() {
                                controller.onUserCenterAction(a);
                            }
                        });
                    }
                }
            });
        }
    }

    /** 未配置 userCenterUrl 时的最小回退(不展示游戏小号;仅切换小号/退出登录)。 */
    private String userCenterFallbackHtml() {
        return "<!doctype html><html><head><meta charset='utf-8'>"
                + "<meta name='viewport' content='width=device-width,initial-scale=1'>"
                + "<style>body{margin:0;font-family:sans-serif;background:#f5f6f8;color:#25272b}"
                + ".card{background:#fff;margin:16px;border-radius:8px;overflow:hidden}"
                + ".row{padding:16px;border-bottom:1px solid #f0f1f4;font-size:16px}.row:active{background:#f7f7f7}"
                + ".tip{color:#9aa0a8;font-size:13px;margin:16px;line-height:1.6}</style></head><body>"
                + "<div class='card'>"
                + "<div class='row' onclick=\"UserCenter.postAccountAction('switch_account')\">切换小号</div>"
                + "<div class='row' onclick=\"UserCenter.postAccountAction('logout')\">退出登录</div>"
                + "</div>"
                + "<div class='tip'>用户中心由平台 H5 提供(未配置 URL,当前为最小回退)。</div>"
                + "</body></html>";
    }

    private android.graphics.drawable.GradientDrawable topRounded(int color, int r) {
        android.graphics.drawable.GradientDrawable g = new android.graphics.drawable.GradientDrawable();
        g.setColor(color);
        g.setCornerRadii(new float[]{r, r, r, r, 0, 0, 0, 0});
        return g;
    }

    // ===== 站内网页层(协议页/通用 H5,#1)=====

    private static final String SDK_VERSION_UA = "2.0.0";
    private static final String PROTOCOL_BASE = "https://p.xingninghuyu.com/agreement/";

    /** 协议名加可点链接 → 站内网页层。 */
    private void linkProtocol(android.text.SpannableString sp, String full, final String label, final String path) {
        int i = full.indexOf(label);
        if (i < 0) {
            return;
        }
        sp.setSpan(new android.text.style.ClickableSpan() {
            @Override
            public void onClick(View widget) {
                openWebOverlay(PROTOCOL_BASE + path, label.replaceAll("[《》]", ""));
            }

            @Override
            public void updateDrawState(android.text.TextPaint ds) {
                ds.setColor(UiKit.PRIMARY_DEEP);
                ds.setUnderlineText(false);
            }
        }, i, i + label.length(), android.text.Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
    }

    /**
     * 站内 WebView 网页层(协议页等):标题栏 + 关闭按钮;链接站内加载、不跳外部浏览器;
     * UserAgent 追加 {@code M5755Sdk/<版本>} 供后端/H5 识别。直接叠在当前层之上(不占用 overlay 槽)。
     */
    @SuppressWarnings("SetJavaScriptEnabled")
    private void openWebOverlay(String url, String title) {
        final ViewGroup root = (ViewGroup) host.findViewById(android.R.id.content);
        LinearLayout panel = new LinearLayout(host);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setBackgroundColor(0xFFFFFFFF);
        panel.setClickable(true);

        LinearLayout bar = new LinearLayout(host);
        bar.setOrientation(LinearLayout.HORIZONTAL);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setBackgroundColor(0xFFF5F6F8);
        bar.setPadding(UiKit.dp(host, 16), UiKit.dp(host, 12), UiKit.dp(host, 12), UiKit.dp(host, 12));
        TextView t = new TextView(host);
        t.setText(title);
        t.setTextSize(16);
        t.setTextColor(0xFF111111);
        TextView close = new TextView(host);
        close.setText("✕");
        close.setTextSize(18);
        close.setTextColor(0xFF747880);
        close.setPadding(UiKit.dp(host, 8), 0, UiKit.dp(host, 8), 0);
        close.setClickable(true);
        bar.addView(t, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        bar.addView(close);
        panel.addView(bar, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));

        final android.webkit.WebView web = new android.webkit.WebView(host);
        android.webkit.WebSettings ws = web.getSettings();
        ws.setJavaScriptEnabled(true);
        ws.setAllowFileAccess(false);
        ws.setAllowFileAccessFromFileURLs(false);
        ws.setAllowUniversalAccessFromFileURLs(false);
        ws.setUserAgentString(ws.getUserAgentString() + " M5755Sdk/" + SDK_VERSION_UA);
        // §1.13 套加载态(占位 + 就绪淡入 + 失败重试);loadableWeb 内部 setWebViewClient + loadUrl
        panel.addView(loadableWeb(web, url, false), new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f));

        final LinearLayout layer = panel;
        root.addView(layer, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        close.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                web.loadUrl("about:blank");
                root.removeView(layer);
                web.destroy();
            }
        });
    }

    /**
     * 给远程加载的 WebView 套加载态(07 §1.13):占位(`WEAK` 底 + 居中品牌动效徽标——轨道环旋转 +
     * 「5755」静止)盖白屏,`onPageFinished` 把 WebView 淡入揭示,`onReceivedError` → 隐藏徽标、
     * 显「加载失败」+「重试」。返回应加入容器的 FrameLayout(含 web + 占位层);内部完成
     * setWebViewClient + loadUrl + 重试。旋转为 §1.11 为 WebView 加载态保留的品牌 spinner 例外;
     * 就绪/失败/关抽屉即停转,无固定超时。仅用于远程 loadUrl;瞬时本地 loadData(回退页)不走此辅助。
     */
    private FrameLayout loadableWeb(final android.webkit.WebView web, final String url, final boolean allowPaySchemes) {
        final FrameLayout container = new FrameLayout(host);
        container.addView(web, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));

        final FrameLayout placeholder = new FrameLayout(host);
        placeholder.setBackgroundColor(UiKit.WEAK);
        placeholder.setClickable(true); // 吃掉点击,避免穿透到加载中的页面
        // 加载中态:品牌动效徽标——轨道环绕中心旋转 + 「5755」静止(160dp,§1.13);失败态隐藏、改显文字+重试
        final int badgeW = UiKit.dp(host, 160);
        final int badgeH = badgeW * 160 / 240; // 维持原 SVG 画布 3:2
        final FrameLayout badge = new FrameLayout(host);
        final android.widget.ImageView orbit = new android.widget.ImageView(host);
        orbit.setImageResource(com.m5755.operate.R.drawable.m5755_web_loading_orbit);
        orbit.setScaleType(android.widget.ImageView.ScaleType.FIT_XY);
        orbit.setPivotX(badgeW / 2f);         // 轨道中心 x = 120/240
        orbit.setPivotY(badgeH * 67f / 160f); // 轨道中心 y = 67/160
        badge.addView(orbit, new FrameLayout.LayoutParams(badgeW, badgeH));
        final android.widget.ImageView mark = new android.widget.ImageView(host);
        mark.setImageResource(com.m5755.operate.R.drawable.m5755_web_loading_brand);
        mark.setScaleType(android.widget.ImageView.ScaleType.FIT_XY);
        badge.addView(mark, new FrameLayout.LayoutParams(badgeW, badgeH));
        placeholder.addView(badge, new FrameLayout.LayoutParams(badgeW, badgeH, Gravity.CENTER));
        // 轨道匀速旋转(1.8s/圈、线性、无限循环);§1.11 为 WebView 加载态保留的品牌 spinner 例外
        final android.animation.ObjectAnimator spin =
                android.animation.ObjectAnimator.ofFloat(orbit, "rotation", 0f, 360f);
        spin.setDuration(1800);
        spin.setInterpolator(new android.view.animation.LinearInterpolator());
        spin.setRepeatCount(android.animation.ValueAnimator.INFINITE);
        orbit.addOnAttachStateChangeListener(new View.OnAttachStateChangeListener() {
            public void onViewAttachedToWindow(View v) {}
            public void onViewDetachedFromWindow(View v) { spin.cancel(); } // 关抽屉即停,免泄漏
        });
        spin.start(); // 占位默认即加载态,开转;onPageFinished/失败时 cancel
        final TextView msg = new TextView(host);
        msg.setTextSize(14);
        msg.setTextColor(UiKit.MUTED);
        msg.setVisibility(View.GONE); // 仅失败态显示「加载失败」
        placeholder.addView(msg, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.CENTER));
        final TextView retry = new TextView(host);
        retry.setText("重试");
        retry.setTextSize(14);
        retry.getPaint().setFakeBoldText(true);
        retry.setTextColor(UiKit.PRIMARY_DEEP);
        retry.setGravity(Gravity.CENTER);
        retry.setPadding(UiKit.dp(host, 22), UiKit.dp(host, 7), UiKit.dp(host, 22), UiKit.dp(host, 7));
        retry.setBackground(UiKit.roundedStroke(UiKit.WHITE, UiKit.dp(host, 8), UiKit.PRIMARY_DEEP, UiKit.dp(host, 1)));
        retry.setVisibility(View.GONE);
        FrameLayout.LayoutParams retryLp = new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.CENTER);
        retryLp.topMargin = UiKit.dp(host, 42);
        placeholder.addView(retry, retryLp);
        container.addView(placeholder, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));

        final boolean[] errored = {false};
        final Runnable showLoading = new Runnable() {
            public void run() {
                errored[0] = false;
                badge.setVisibility(View.VISIBLE);
                if (!spin.isStarted()) spin.start();
                msg.setVisibility(View.GONE);
                retry.setVisibility(View.GONE);
                placeholder.setVisibility(View.VISIBLE);
                web.setAlpha(0f);
            }
        };
        final Runnable showError = new Runnable() {
            public void run() {
                errored[0] = true;
                badge.setVisibility(View.GONE);
                spin.cancel();
                msg.setText("加载失败");
                msg.setVisibility(View.VISIBLE);
                retry.setVisibility(View.VISIBLE);
                placeholder.setVisibility(View.VISIBLE);
                web.setAlpha(0f);
            }
        };
        retry.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                showLoading.run();
                web.loadUrl(url);
            }
        });

        web.setAlpha(0f);
        // JS dialog 支持:远程页 alert/confirm 弹原生对话框(uc SPA 退出登录二次确认、收银台支付提示等)。
        // 无 WebChromeClient 时 WebView 的 confirm() 默认返回 false → 远程页的二次确认静默失效。
        // 仅 onJsAlert/onJsConfirm;不实现 onShowFileChooser → 文件选择仍按 01 §4.2 排除。
        web.setWebChromeClient(new android.webkit.WebChromeClient() {
            @Override
            public boolean onJsConfirm(android.webkit.WebView v, String u, String message, android.webkit.JsResult r) {
                jsDialog(message, true, r);
                return true;
            }
            @Override
            public boolean onJsAlert(android.webkit.WebView v, String u, String message, android.webkit.JsResult r) {
                jsDialog(message, false, r);
                return true;
            }
        });
        web.setWebViewClient(new android.webkit.WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(android.webkit.WebView v, String u) {
                if (u != null && (u.startsWith("http://") || u.startsWith("https://"))) {
                    v.loadUrl(u); // 站内加载,不外跳系统浏览器
                    return true;
                }
                // 支付域受限外跳例外(仅收银台 allowPaySchemes、仅白名单 scheme;01 §4.2 例外 / ADR-0014):
                // startActivity(VIEW) 直拉渠道 App,未安装 catch ANFE 泛化兜底(零 queries、不点名渠道守 07 §0.2)。
                if (allowPaySchemes && isPaySchemeWhitelisted(u)) {
                    try {
                        host.startActivity(new android.content.Intent(
                                android.content.Intent.ACTION_VIEW, android.net.Uri.parse(u)));
                    } catch (Exception e) {
                        // 未安装(ActivityNotFoundException)+ SecurityException / 畸形 Uri 等一律兜底:
                        // AAR 寄生游戏进程,任何未捕获异常都会带崩游戏,故放宽到 Exception(泛化提示不点名渠道)。
                        toast("未检测到所选支付应用,请安装后重试或换一种支付方式");
                    }
                    return true;
                }
                return true; // 白名单外的非 http scheme:吞掉(通用外跳仍禁,01 §4.2)
            }
            @Override
            public void onPageFinished(android.webkit.WebView v, String u) {
                if (errored[0]) {
                    return; // 出错后停在错误态,不淡入错误页
                }
                spin.cancel(); // 页面就绪,停转
                placeholder.setVisibility(View.GONE);
                v.animate().alpha(1f).setDuration(200).start(); // §1.11 允许的轻微淡入
            }
            @Override
            public void onReceivedError(android.webkit.WebView v, android.webkit.WebResourceRequest req,
                                        android.webkit.WebResourceError err) {
                if (req != null && req.isForMainFrame()) {
                    showError.run(); // 主框架加载失败(API 23+)
                }
            }
            @Override
            @SuppressWarnings("deprecation")
            public void onReceivedError(android.webkit.WebView v, int code, String desc, String failingUrl) {
                if (url.equals(failingUrl)) {
                    showError.run(); // API <23:按主 url 过滤,避免子资源误报
                }
            }
        });
        web.loadUrl(url);
        return container;
    }

    /** WebChromeClient 的 JS dialog → 原生 AlertDialog;confirm 有「取消」、alert 仅「确定」(取消即确定)。 */
    private void jsDialog(String message, final boolean isConfirm, final android.webkit.JsResult result) {
        android.app.AlertDialog.Builder b = new android.app.AlertDialog.Builder(host)
                .setMessage(message)
                .setPositiveButton("确定", new android.content.DialogInterface.OnClickListener() {
                    public void onClick(android.content.DialogInterface d, int w) { result.confirm(); }
                })
                .setOnCancelListener(new android.content.DialogInterface.OnCancelListener() {
                    public void onCancel(android.content.DialogInterface d) {
                        if (isConfirm) { result.cancel(); } else { result.confirm(); }
                    }
                });
        if (isConfirm) {
            b.setNegativeButton("取消", new android.content.DialogInterface.OnClickListener() {
                public void onClick(android.content.DialogInterface d, int w) { result.cancel(); }
            });
        }
        b.show();
    }

    /**
     * 支付域外跳 scheme 白名单(01 §4.2 受限例外 / ADR-0014):仅微信、支付宝付款 scheme。
     * 白名单外的非 http scheme 一律不外跳(通用外跳仍永久排除)。package-private 供单测锁收窄、防无声放宽。
     */
    static boolean isPaySchemeWhitelisted(String u) {
        if (u == null) {
            return false;
        }
        return u.startsWith("weixin://")      // 微信(含 weixin://wap/pay)
                || u.startsWith("alipays://")   // 支付宝
                || u.startsWith("alipayqr://"); // 支付宝二维码付款变体
    }

    // ===== 模态构建 =====

    private void mountProtocol() {
        LinearLayout card = UiKit.modalCard(host, 520);
        card.addView(UiKit.title(host, "个人信息保护引导"));
        String bodyText = "本游戏接入 5755 SDK。为提供游戏资源加载、联网、账号安全、实名防沉迷、支付、用户中心和诊断能力,"
                + "SDK 需要处理必要的设备信息、网络信息、当前游戏小号信息和日志信息。\n\n"
                + "请阅读《用户注册协议》《用户隐私协议》《儿童隐私保护指引》《第三方信息共享清单》。同意后进入账号登录。";
        TextView body = UiKit.hint(host, "");
        android.text.SpannableString sp = new android.text.SpannableString(bodyText);
        linkProtocol(sp, bodyText, "《用户注册协议》", "register");
        linkProtocol(sp, bodyText, "《用户隐私协议》", "privacy");
        linkProtocol(sp, bodyText, "《儿童隐私保护指引》", "children");
        linkProtocol(sp, bodyText, "《第三方信息共享清单》", "third-party");
        body.setText(sp);
        body.setMovementMethod(android.text.method.LinkMovementMethod.getInstance());
        body.setHighlightColor(0x00000000);
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
                mountPasswordLogin();
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
        phone.setFilters(new android.text.InputFilter[]{new android.text.InputFilter.LengthFilter(11)}); // 手机号 11 位上限
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
                if (!p.matches("^1\\d{10}$")) {
                    toast("请输入正确的 11 位手机号");
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

    private void mountPasswordLogin() {
        protocolChecked = false;
        LinearLayout card = UiKit.modalCard(host, 340);
        LinearLayout tabs = new LinearLayout(host);
        tabs.setOrientation(LinearLayout.HORIZONTAL);
        TextView tabSms = tab(host, "验证码登录", false);
        TextView tabPwd = tab(host, "密码登录", true);
        tabs.addView(tabSms);
        tabs.addView(tabPwd);
        card.addView(tabs);
        tabSms.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                mountLogin();
            }
        });

        TextView tip = new TextView(host);
        tip.setText("可使用手机号或账号密码登录");
        tip.setTextSize(12);
        tip.setTextColor(UiKit.MUTED);
        LinearLayout.LayoutParams tipLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        tipLp.topMargin = UiKit.dp(host, 14);
        card.addView(tip, tipLp);

        final android.widget.EditText acct = UiKit.input(host, "请输入手机号码");
        card.addView(acct);

        // 密码行 + 显示切换
        LinearLayout pwdRow = new LinearLayout(host);
        pwdRow.setOrientation(LinearLayout.HORIZONTAL);
        pwdRow.setGravity(Gravity.CENTER_VERTICAL);
        pwdRow.setBackground(UiKit.rounded(UiKit.WEAK, UiKit.dp(host, 6)));
        pwdRow.setPadding(0, 0, UiKit.dp(host, 10), 0);
        LinearLayout.LayoutParams pwdRowLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 44));
        pwdRowLp.topMargin = UiKit.dp(host, 12);
        final android.widget.EditText pwd = new android.widget.EditText(host);
        pwd.setHint("请输入密码");
        pwd.setSingleLine(true);
        pwd.setTextSize(14);
        pwd.setTextColor(UiKit.TEXT);
        pwd.setHintTextColor(UiKit.HINT);
        pwd.setBackgroundColor(0x00000000);
        pwd.setPadding(UiKit.dp(host, 14), 0, UiKit.dp(host, 8), 0);
        pwd.setInputType(android.text.InputType.TYPE_CLASS_TEXT | android.text.InputType.TYPE_TEXT_VARIATION_PASSWORD);
        pwdRow.addView(pwd, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1));
        final TextView showBtn = new TextView(host);
        showBtn.setText("显示");
        showBtn.setTextSize(13);
        showBtn.setTextColor(UiKit.PRIMARY_DEEP);
        showBtn.setGravity(Gravity.CENTER);
        showBtn.setClickable(true);
        pwdRow.addView(showBtn, new LinearLayout.LayoutParams(UiKit.dp(host, 80), ViewGroup.LayoutParams.MATCH_PARENT));
        card.addView(pwdRow, pwdRowLp);
        final boolean[] shown = {false};
        showBtn.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                shown[0] = !shown[0];
                pwd.setInputType(android.text.InputType.TYPE_CLASS_TEXT
                        | (shown[0] ? android.text.InputType.TYPE_TEXT_VARIATION_VISIBLE_PASSWORD
                        : android.text.InputType.TYPE_TEXT_VARIATION_PASSWORD));
                pwd.setSelection(pwd.getText().length());
                showBtn.setText(shown[0] ? "隐藏" : "显示");
            }
        });

        TextView loginBtn = UiKit.primaryButton(host, "登录");
        card.addView(loginBtn);
        card.addView(protocolRow());

        loginBtn.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                if (!protocolChecked) {
                    toast("请先阅读并同意协议");
                    return;
                }
                final String a = acct.getText().toString().trim();
                final String p = pwd.getText().toString();
                if (a.isEmpty()) {
                    toast("请输入手机号或账号");
                    return;
                }
                if (p.isEmpty()) {
                    toast("请输入密码");
                    return;
                }
                background.execute(new Runnable() {
                    public void run() {
                        controller.submitPasswordLogin(a, p);
                    }
                });
            }
        });
        mount(card);
    }

    @Override
    public void showDeviceVerify(final String loginAccount) {
        main.post(new Runnable() {
            public void run() {
                mountDeviceVerify(loginAccount);
            }
        });
    }

    private void mountDeviceVerify(final String loginAccount) {
        LinearLayout card = UiKit.modalCard(host, 420);
        card.addView(UiKit.title(host, "设备安全验证"));
        LinearLayout.LayoutParams hintLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        hintLp.topMargin = UiKit.dp(host, 14);
        card.addView(UiKit.hint(host, "设备首次账号密码登录时,需进行绑定手机号短信验证。"), hintLp);

        LinearLayout codeRow = new LinearLayout(host);
        codeRow.setOrientation(LinearLayout.HORIZONTAL);
        codeRow.setGravity(Gravity.CENTER_VERTICAL);
        codeRow.setBackground(UiKit.rounded(UiKit.WEAK, UiKit.dp(host, 6)));
        codeRow.setPadding(0, 0, UiKit.dp(host, 10), 0);
        LinearLayout.LayoutParams crLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 44));
        crLp.topMargin = UiKit.dp(host, 14);
        final android.widget.EditText code = new android.widget.EditText(host);
        code.setHint("请输入验证码");
        code.setSingleLine(true);
        code.setTextSize(14);
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
        card.addView(codeRow, crLp);

        TextView submit = UiKit.primaryButton(host, "提交");
        card.addView(submit);
        sendCodeButton.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                sendCodeButton.setEnabled(false);
                sendCodeButton.setText("发送中");
                sendCodeButton.setTextColor(UiKit.HINT);
                background.execute(new Runnable() {
                    public void run() {
                        controller.requestCode(loginAccount); // 发往账户绑定手机号
                    }
                });
            }
        });
        submit.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                final String c = code.getText().toString().trim();
                if (c.isEmpty()) {
                    toast("请输入验证码");
                    return;
                }
                background.execute(new Runnable() {
                    public void run() {
                        controller.submitDeviceVerify(c);
                    }
                });
            }
        });
        mount(card);
    }

    /** 协议勾选行(登录/密码登录共用)。 */
    private LinearLayout protocolRow() {
        LinearLayout checkRow = new LinearLayout(host);
        checkRow.setOrientation(LinearLayout.HORIZONTAL);
        checkRow.setGravity(Gravity.CENTER_VERTICAL);
        checkRow.setClickable(true);
        LinearLayout.LayoutParams checkLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        checkLp.topMargin = UiKit.dp(host, 12);
        checkRow.setLayoutParams(checkLp);
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
        LinearLayout.LayoutParams ctLp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        ctLp.leftMargin = UiKit.dp(host, 8);
        checkRow.addView(box);
        checkRow.addView(checkText, ctLp);
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
        return checkRow;
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

    /** 退出游戏确认弹窗(07 §10c,#30):由 facade 直接调用。 */
    public void showQuitConfirm(final Runnable onConfirm, final Runnable onCancel) {
        main.post(new Runnable() {
            public void run() {
                LinearLayout card = UiKit.modalCard(host, 420);
                card.addView(UiKit.title(host, "退出游戏"));
                LinearLayout.LayoutParams hintLp = new LinearLayout.LayoutParams(
                        ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
                hintLp.topMargin = UiKit.dp(host, 14);
                card.addView(UiKit.hint(host, "确认退出游戏吗?"), hintLp);
                LinearLayout row = new LinearLayout(host);
                row.setOrientation(LinearLayout.HORIZONTAL);
                LinearLayout.LayoutParams rowLp = new LinearLayout.LayoutParams(
                        ViewGroup.LayoutParams.MATCH_PARENT, UiKit.dp(host, 48));
                rowLp.topMargin = UiKit.dp(host, 18);
                TextView cancel = UiKit.secondaryButton(host, "取消");
                TextView confirm = UiKit.primaryButton(host, "退出");
                LinearLayout.LayoutParams half = new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1);
                half.rightMargin = UiKit.dp(host, 10);
                LinearLayout.LayoutParams half2 = new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1);
                half2.leftMargin = UiKit.dp(host, 10);
                cancel.setLayoutParams(half);
                confirm.setLayoutParams(half2);
                row.addView(cancel);
                row.addView(confirm);
                card.addView(row, rowLp);
                cancel.setOnClickListener(new View.OnClickListener() {
                    public void onClick(View v) {
                        dismiss();
                        if (onCancel != null) {
                            onCancel.run();
                        }
                    }
                });
                confirm.setOnClickListener(new View.OnClickListener() {
                    public void onClick(View v) {
                        dismiss();
                        if (onConfirm != null) {
                            onConfirm.run();
                        }
                    }
                });
                mount(card);
            }
        });
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
