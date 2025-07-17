package wechat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mdp/qrterminal/v3"
)

// Client 微信客户端
type Client struct {
	baseURL  string
	adminKey string
	authKey  string
}

// NewClient 创建新的微信客户端
func NewClient() *Client {
	return &Client{
		baseURL:  BaseURL,
		adminKey: getAdminKey(),
	}
}

// getAdminKey 获取管理员密钥
func getAdminKey() string {
	if key := os.Getenv("WECHAT_ADMIN_KEY"); key != "" {
		return key
	}
	return "12345"
}

// Login 执行完整的登录流程
func (c *Client) Login() (string, error) {
	// 生成授权码
	authKey, err := c.generateAuthKey()
	if err != nil {
		return "", fmt.Errorf("生成授权码失败: %v", err)
	}
	c.authKey = authKey

	// 获取登录二维码
	qrCode, uuid, err := c.getLoginQrCode()
	if err != nil {
		return "", fmt.Errorf("获取二维码失败: %v", err)
	}

	// 显示二维码
	fmt.Println("\n📲 请使用微信扫描下方二维码:")
	fmt.Println("==================================")
	qrterminal.Generate(qrCode, qrterminal.M, os.Stdout)
	fmt.Println("==================================")

	// 等待登录
	wxid, err := c.waitForLogin(uuid)
	if err != nil {
		return "", fmt.Errorf("登录失败: %v", err)
	}

	return wxid, nil
}

// generateAuthKey 生成授权码
func (c *Client) generateAuthKey() (string, error) {
	url := c.baseURL + "/admin/GenAuthKey1?key=" + c.adminKey

	reqData := GenAuthKeyRequest{
		Count: 1,
		Days:  30,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("序列化请求数据失败: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	var authResp GenAuthKeyResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if authResp.Code != 200 {
		return "", fmt.Errorf("API错误 (Code: %d): %s", authResp.Code, authResp.Text)
	}

	if len(authResp.Data) == 0 {
		return "", fmt.Errorf("授权码数据为空")
	}

	return authResp.Data[0], nil
}

// getLoginQrCode 获取登录二维码
func (c *Client) getLoginQrCode() (string, string, error) {
	apiUrl := c.baseURL + "/login/GetLoginQrCodeNew?key=" + c.authKey

	reqData := GetLoginQrCodeRequest{
		Check: false,
		Proxy: "",
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", "", fmt.Errorf("序列化请求数据失败: %v", err)
	}

	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取响应失败: %v", err)
	}

	var qrResp GetLoginQrCodeResponse
	if err := json.Unmarshal(body, &qrResp); err != nil {
		return "", "", fmt.Errorf("解析响应失败: %v", err)
	}

	if qrResp.Code != 200 {
		return "", "", fmt.Errorf("API错误 (Code: %d): %s", qrResp.Code, qrResp.Text)
	}

	qrCodeUrl := qrResp.Data.QrCodeUrl
	parsedUrl, err := url.Parse(qrCodeUrl)
	if err != nil {
		return "", "", fmt.Errorf("解析二维码URL失败: %v", err)
	}

	actualQrData := parsedUrl.Query().Get("url")
	if actualQrData == "" {
		return "", "", fmt.Errorf("未找到二维码数据")
	}

	parts := strings.Split(actualQrData, "/")
	uuid := parts[len(parts)-1]

	return actualQrData, uuid, nil
}

// waitForLogin 等待登录完成
func (c *Client) waitForLogin(uuid string) (string, error) {
	url := c.baseURL + "/login/CheckLoginStatus"

	reqData := CheckLoginStatusRequest{
		AuthKey: c.authKey,
		UUID:    uuid,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("序列化请求数据失败: %v", err)
	}

	for i := 0; i < 60; i++ {
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		var statusResp CheckLoginStatusResponse
		if err := json.Unmarshal(body, &statusResp); err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		switch statusResp.Data.Status {
		case LoginStatusWaiting:
			fmt.Print("⏳ 等待扫码...")
		case LoginStatusScanned:
			fmt.Print("📱 已扫码，等待确认...")
		case LoginStatusSuccess:
			fmt.Print("✅ 登录成功!")
			return statusResp.Data.Wxid, nil
		case LoginStatusFailed:
			return "", fmt.Errorf("登录失败")
		case LoginStatusTimeout:
			return "", fmt.Errorf("登录超时")
		}

		time.Sleep(5 * time.Second)
		fmt.Print(".")
	}

	return "", fmt.Errorf("等待登录超时")
}