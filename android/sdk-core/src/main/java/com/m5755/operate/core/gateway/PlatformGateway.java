package com.m5755.operate.core.gateway;

/**
 * SDK 内部平台网关边界(02 术语「平台网关边界」):状态机只依赖它表达各类结果,
 * 不直接承担 HTTP 拼装、签名或环境选择。真实实现见 {@link HttpPlatformGateway};
 * 测试注入内存假实现。所有方法为阻塞调用,由上层在后台线程执行。
 */
public interface PlatformGateway {

    Results.Config fetchConfig(String gameId, String sdkVersion, String packageName,
                               String channelId, String channelSource);

    Results.Sms requestSms(String gameId, String loginAccount);

    Results.Login login(String gameId, String loginAccount, String credential,
                        String channelId, String channelSource);

    /** #15 账户有效检查(凭据走请求头)。 */
    Results.AccountCheck checkAccount(String gameId, String platformAccountId, String platformToken);

    /** #16 实名状态检查 / 提交。 */
    Results.RealName getRealName(String gameId, String platformAccountId, String platformToken);

    Results.RealName submitRealName(String gameId, String platformAccountId, String platformToken,
                                    String realName, String idNumber);

    /** #17 小号列表 / 添加 / 设默认。 */
    Results.SubaccountList listSubaccounts(String gameId, String platformAccountId, String platformToken);

    Results.SubaccountOp createSubaccount(String gameId, String platformAccountId, String platformToken);

    Results.SubaccountOp setDefaultSubaccount(String gameId, String platformAccountId, String platformToken, String account);

    /** #18 小号登录(签发游戏侧 account/token)。 */
    Results.SubaccountLogin loginSubaccount(String gameId, String platformAccountId, String platformToken, String account);
}
