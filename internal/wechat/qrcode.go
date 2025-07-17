package wechat

import (
	"fmt"
	"os"

	"github.com/mdp/qrterminal/v3"
)

// QRCodeGenerator 二维码生成器
type QRCodeGenerator struct{}

// NewQRCodeGenerator 创建二维码生成器
func NewQRCodeGenerator() *QRCodeGenerator {
	return &QRCodeGenerator{}
}

// DisplayQRCode 显示二维码
func (q *QRCodeGenerator) DisplayQRCode(qrData string) {
	fmt.Println("\n📲 请使用微信扫描下方二维码:")
	fmt.Println("==================================")
	qrterminal.Generate(qrData, qrterminal.M, os.Stdout)
	fmt.Println("==================================")
}

// GenerateQRCodeString 生成二维码字符串
func (q *QRCodeGenerator) GenerateQRCodeString(qrData string) string {
	// 可以用于获取二维码的字符串表示，用于日志或其他用途
	return qrData
}