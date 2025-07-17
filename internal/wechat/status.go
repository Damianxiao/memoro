package wechat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// StatusChecker 状态检查器
type StatusChecker struct {
	client   *Client
	interval time.Duration
}

// NewStatusChecker 创建状态检查器
func NewStatusChecker(client *Client) *StatusChecker {
	return &StatusChecker{
		client:   client,
		interval: 30 * time.Second,
	}
}

// CheckCurrentStatus 检查当前登录状态
func (s *StatusChecker) CheckCurrentStatus() (bool, error) {
	// 尝试调用GetLoginStatus API来检查状态
	url := s.client.baseURL + "/login/GetLoginStatus"
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("创建请求失败: %v", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("读取响应失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("解析响应失败: %v", err)
	}

	// WeChatPadPro API 返回格式：
	// {"Code": 200, "Data": {...}, "Text": "success"} - 已登录
	// {"Code": -2, "Data": null, "Text": "该链接不存在！"} - 未认证/未登录
	// {"Code": 其他, "Data": null, "Text": "error message"} - 其他错误
	
	if code, ok := result["Code"].(float64); ok {
		if code == 200 {
			// 进一步检查Data字段是否包含有效的登录信息
			if data, ok := result["Data"]; ok && data != nil {
				return true, nil
			}
		}
	}

	return false, nil
}

// StartMonitoring 启动状态监控
func (s *StatusChecker) StartMonitoring(onStatusChange func(bool)) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	var lastStatus bool
	
	for {
		select {
		case <-ticker.C:
			status, err := s.CheckCurrentStatus()
			if err != nil {
				fmt.Printf("状态检查失败: %v\n", err)
				continue
			}

			if status != lastStatus {
				lastStatus = status
				onStatusChange(status)
			}
		}
	}
}

// TriggerLogin 触发登录流程
func (s *StatusChecker) TriggerLogin() error {
	fmt.Println("🔄 检测到微信登录失效，正在重新登录...")
	
	wxid, err := s.client.Login()
	if err != nil {
		return fmt.Errorf("自动登录失败: %v", err)
	}

	fmt.Printf("✅ 自动登录成功，wxid: %s\n", wxid)
	
	// 这里可以添加更新配置文件的逻辑
	if err := s.updateConfig(wxid); err != nil {
		fmt.Printf("⚠️  配置更新失败: %v\n", err)
	}

	return nil
}

// updateConfig 更新配置文件
func (s *StatusChecker) updateConfig(wxid string) error {
	// TODO: 实现配置文件更新逻辑
	fmt.Printf("📝 需要更新配置文件中的 wxid: %s\n", wxid)
	return nil
}