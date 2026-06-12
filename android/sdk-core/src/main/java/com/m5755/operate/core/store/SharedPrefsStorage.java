package com.m5755.operate.core.store;

import android.content.Context;
import android.content.SharedPreferences;

/**
 * {@link Storage} 的 SharedPreferences 实现。文件名 {@code m5755_operate_min}
 * (验收口径 08 §2.2:登录成功后 shared_prefs/m5755_operate_min.xml 出现
 * platform_account_id / platform_token / account)。
 */
public final class SharedPrefsStorage implements Storage {

    private static final String FILE = "m5755_operate_min";
    private static final String K_PROTOCOL = "protocol_consented_version";
    private static final String K_PA_ID = "platform_account_id";
    private static final String K_TOKEN = "platform_token";
    private static final String K_ACCOUNT = "account";

    private final SharedPreferences sp;

    public SharedPrefsStorage(Context context) {
        this.sp = context.getApplicationContext().getSharedPreferences(FILE, Context.MODE_PRIVATE);
    }

    @Override
    public boolean isProtocolConsented(String protocolVersion) {
        return protocolVersion != null && protocolVersion.equals(sp.getString(K_PROTOCOL, null));
    }

    @Override
    public void setProtocolConsented(String protocolVersion) {
        sp.edit().putString(K_PROTOCOL, protocolVersion).apply();
    }

    @Override
    public boolean hasSession() {
        return sp.getString(K_TOKEN, null) != null;
    }

    @Override
    public void saveSession(String platformAccountId, String platformToken, String account) {
        sp.edit()
                .putString(K_PA_ID, platformAccountId)
                .putString(K_TOKEN, platformToken)
                .putString(K_ACCOUNT, account)
                .apply();
    }

    @Override
    public void clearSession() {
        sp.edit().remove(K_PA_ID).remove(K_TOKEN).remove(K_ACCOUNT).remove(K_SUB_TOKEN).apply();
    }

    private static final String K_SUB_TOKEN = "sub_token";

    @Override
    public String getPlatformAccountId() {
        return sp.getString(K_PA_ID, null);
    }

    @Override
    public String getPlatformToken() {
        return sp.getString(K_TOKEN, null);
    }

    @Override
    public String getAccount() {
        return sp.getString(K_ACCOUNT, null);
    }

    @Override
    public void saveSubaccount(String account, String subaccountToken) {
        sp.edit().putString(K_ACCOUNT, account).putString(K_SUB_TOKEN, subaccountToken).apply();
    }

    @Override
    public String getSubaccountToken() {
        return sp.getString(K_SUB_TOKEN, null);
    }
}
