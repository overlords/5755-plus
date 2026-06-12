package com.m5755.operate.api;

/**
 * 区服角色归属字段基类(公开 API)。{@link RoleInfo} 与 {@link Order} 共享。
 */
public class RoleMeta {

    protected String serverId = "-1";
    protected String serverName = "-1";
    protected String roleId = "";
    protected String roleName = "";
    protected String roleLevel = "";

    public String getServerId() {
        return serverId;
    }

    public void setServerId(String v) {
        this.serverId = v;
    }

    public String getServerName() {
        return serverName;
    }

    public void setServerName(String v) {
        this.serverName = v;
    }

    public String getRoleId() {
        return roleId;
    }

    public void setRoleId(String v) {
        this.roleId = v;
    }

    public String getRoleName() {
        return roleName;
    }

    public void setRoleName(String v) {
        this.roleName = v;
    }

    public String getRoleLevel() {
        return roleLevel;
    }

    public void setRoleLevel(String v) {
        this.roleLevel = v;
    }
}
