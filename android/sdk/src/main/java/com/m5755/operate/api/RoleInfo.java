package com.m5755.operate.api;

/**
 * 角色资料(公开 API,05 §1)。所有字段必填;确实不存在的字段传 {@code "-1"},但 {@code roleId} 不允许 {@code "-1"}。
 */
public final class RoleInfo extends RoleMeta {

    private String roleCe = "-1";
    private String roleStage = "-1";
    private String roleRechargeAmount = "-1";
    private String roleGuild = "-1";

    public String getRoleCe() {
        return roleCe;
    }

    public void setRoleCe(String v) {
        this.roleCe = v;
    }

    public String getRoleStage() {
        return roleStage;
    }

    public void setRoleStage(String v) {
        this.roleStage = v;
    }

    public String getRoleRechargeAmount() {
        return roleRechargeAmount;
    }

    public void setRoleRechargeAmount(String v) {
        this.roleRechargeAmount = v;
    }

    public String getRoleGuild() {
        return roleGuild;
    }

    public void setRoleGuild(String v) {
        this.roleGuild = v;
    }
}
