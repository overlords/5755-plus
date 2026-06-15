package com.m5755.operate.api;

/**
 * 支付订单入参(公开 API,05 §2.2)。必须显式绑定金额、商品、CP 订单号与区服角色归属。
 */
public final class Order extends RoleMeta {

    private String amount = "";
    private String cpOrderId = "";
    private String commodity = "";

    /** 订单金额:两位小数字符串 ^\d+\.\d{2}$(如 "328.00"),单位元,> 0(04 §2.13)。 */
    public String getAmount() {
        return amount;
    }

    public void setAmount(String v) {
        this.amount = v == null ? "" : v;
    }

    public String getCpOrderId() {
        return cpOrderId;
    }

    public void setCpOrderId(String v) {
        this.cpOrderId = v;
    }

    public String getCommodity() {
        return commodity;
    }

    public void setCommodity(String v) {
        this.commodity = v;
    }
}
