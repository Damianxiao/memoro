package wechat

// GenAuthKeyRequest 生成授权码请求
type GenAuthKeyRequest struct {
	Count int `json:"Count"`
	Days  int `json:"Days"`
}

// GenAuthKeyResponse 生成授权码响应
type GenAuthKeyResponse struct {
	Code int      `json:"Code"`
	Data []string `json:"Data"`
	Text string   `json:"Text"`
}

// GetLoginQrCodeRequest 获取登录二维码请求
type GetLoginQrCodeRequest struct {
	Check bool   `json:"Check"`
	Proxy string `json:"Proxy"`
}

// GetLoginQrCodeResponse 获取登录二维码响应
type GetLoginQrCodeResponse struct {
	Code int `json:"Code"`
	Data struct {
		Key       string `json:"Key"`
		QrCodeUrl string `json:"QrCodeUrl"`
		Txt       string `json:"Txt"`
		BaseResp  struct {
			Ret    int         `json:"ret"`
			ErrMsg interface{} `json:"errMsg"`
		} `json:"baseResp"`
	} `json:"Data"`
	Text string `json:"Text"`
}

// CheckLoginStatusRequest 检查登录状态请求
type CheckLoginStatusRequest struct {
	AuthKey string `json:"auth_key"`
	UUID    string `json:"uuid"`
}

// CheckLoginStatusResponse 检查登录状态响应
type CheckLoginStatusResponse struct {
	Code int `json:"Code"`
	Data struct {
		Status int    `json:"status"`
		Wxid   string `json:"wxid"`
		Avatar string `json:"avatar"`
		Name   string `json:"name"`
	} `json:"Data"`
	Text string `json:"Text"`
}

// 登录状态常量
const (
	LoginStatusWaiting = 1 // 等待扫码
	LoginStatusScanned = 2 // 已扫码，等待确认
	LoginStatusSuccess = 3 // 登录成功
	LoginStatusFailed  = 4 // 登录失败
	LoginStatusTimeout = 5 // 超时
)

// 配置常量
const (
	BaseURL = "http://localhost:1239"
)