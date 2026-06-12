package com.m5755.sdk.ui;

import android.content.Context;
import android.graphics.Color;
import android.graphics.drawable.GradientDrawable;
import android.text.InputType;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.TextView;

/**
 * SDK UI 通用控件与颜色常量(07 §1)。程序化构建视图,不依赖 AndroidX,不依赖 XML 布局资源。
 */
final class UiKit {

    // 颜色(07 §1.1)
    static final int PRIMARY = 0xFFFFC936;
    static final int PRIMARY_DEEP = 0xFFF3AD12;
    static final int TEXT = 0xFF25272B;
    static final int MUTED = 0xFF777B83;
    static final int WEAK = 0xFFF2F3F5;
    static final int LINE = 0xFFE8E9EE;
    static final int WHITE = 0xFFFFFFFF;
    static final int BTN_TEXT_ON_PRIMARY = 0xFF5D4300;
    static final int SECONDARY_BG = 0xFFE7E9EF;
    static final int SECONDARY_TEXT = 0xFF6B7078;
    static final int HINT = 0xFFA6A9B0;
    static final int CHECK_TEXT = 0xFF9A9CA3;
    static final int TAB_UNSELECTED = 0xFF61646B;
    static final int MASK = 0x78000000;
    static final int TOAST_BG = 0xB8000000;

    static int dp(Context c, float v) {
        return Math.round(TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, v, c.getResources().getDisplayMetrics()));
    }

    static GradientDrawable rounded(int color, int radiusPx) {
        GradientDrawable g = new GradientDrawable();
        g.setColor(color);
        g.setCornerRadius(radiusPx);
        return g;
    }

    static GradientDrawable roundedStroke(int color, int radiusPx, int strokeColor, int strokePx) {
        GradientDrawable g = rounded(color, radiusPx);
        g.setStroke(strokePx, strokeColor);
        return g;
    }

    static TextView title(Context c, String text) {
        TextView t = new TextView(c);
        t.setText(text);
        t.setTextSize(18);
        t.setTextColor(TEXT);
        t.getPaint().setFakeBoldText(true);
        t.setGravity(Gravity.CENTER);
        return t;
    }

    static TextView hint(Context c, String text) {
        TextView t = new TextView(c);
        t.setText(text);
        t.setTextSize(13);
        t.setTextColor(MUTED);
        t.setLineSpacing(dp(c, 3), 1f);
        return t;
    }

    static EditText input(Context c, String hint) {
        EditText e = new EditText(c);
        e.setHint(hint);
        e.setSingleLine(true);
        e.setTextSize(14);
        e.setTextColor(TEXT);
        e.setHintTextColor(HINT);
        e.setBackground(rounded(WEAK, dp(c, 6)));
        e.setPadding(dp(c, 14), 0, dp(c, 14), 0);
        LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(c, 44));
        lp.topMargin = dp(c, 12);
        e.setLayoutParams(lp);
        return e;
    }

    /** 主按钮:PRIMARY 底、#5D4300 字、6dp 圆角、48dp 高、17sp 粗体(07 §1.4)。 */
    static TextView primaryButton(Context c, String text) {
        TextView b = new TextView(c);
        b.setText(text);
        b.setTextSize(17);
        b.setTextColor(BTN_TEXT_ON_PRIMARY);
        b.getPaint().setFakeBoldText(true);
        b.setGravity(Gravity.CENTER);
        b.setBackground(rounded(PRIMARY, dp(c, 6)));
        b.setClickable(true);
        LinearLayout.LayoutParams lp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(c, 48));
        lp.topMargin = dp(c, 16);
        b.setLayoutParams(lp);
        return b;
    }

    /** 次按钮:#E7E9EF 底、#6B7078 字、6dp 圆角(07 §1.4)。 */
    static TextView secondaryButton(Context c, String text) {
        TextView b = new TextView(c);
        b.setText(text);
        b.setTextSize(15);
        b.setTextColor(SECONDARY_TEXT);
        b.getPaint().setFakeBoldText(true);
        b.setGravity(Gravity.CENTER);
        b.setBackground(rounded(SECONDARY_BG, dp(c, 6)));
        b.setClickable(true);
        return b;
    }

    /** 居中白卡片(07 §1.3):圆角 10dp,内容区左右 24dp。 */
    static LinearLayout modalCard(Context c, int widthDp) {
        LinearLayout card = new LinearLayout(c);
        card.setOrientation(LinearLayout.VERTICAL);
        card.setBackground(rounded(WHITE, dp(c, 10)));
        card.setElevation(dp(c, 18));
        int padH = dp(c, 24);
        card.setPadding(padH, dp(c, 18), padH, dp(c, 24));
        int dm = c.getResources().getDisplayMetrics().widthPixels;
        int want = dp(c, widthDp);
        int max = dm - dp(c, 48);
        card.setMinimumWidth(Math.min(want, max));
        return card;
    }

    private UiKit() {
    }
}
