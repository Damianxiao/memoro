# WeChatPadPro 配置说明

## 服务概述

WeChatPadPro 已成功迁移到 Memoro 项目中，提供微信消息收发的底层服务。

## 服务配置

- **主服务端口**: 1239
- **管理密钥**: 12345
- **微信ID**: wxid_w3a18zqallvs12
- **用户昵称**: toe

## API 接口

### 基础接口
- 健康检查: `GET http://localhost:1239/`
- 登录状态: `POST http://localhost:1239/login/CheckLoginStatus?key={token}`
- 获取二维码: `POST http://localhost:1239/login/GetLoginQrCodeNew?key={token}`

### 消息接口  
- 发送文本: `POST http://localhost:1239/message/SendTextMsg?key={token}`
- 发送图片: `POST http://localhost:1239/message/SendImageMsg?key={token}`
- WebSocket: `ws://localhost:1239/ws/GetSyncMsg?key={token}`

## 数据存储

- **MySQL**: 用户信息和消息记录
- **Redis**: 缓存和会话存储
- **文件存储**: 用户数据和日志

## 使用说明

1. 启动服务: `docker-compose up -d`
2. 检查状态: `docker-compose ps`
3. 查看日志: `docker-compose logs wechatpadpro`
4. 停止服务: `docker-compose down`

## 注意事项

- 首次启动后需要重新扫码登录微信
- 保持手机微信在线以维持连接稳定性
- 定期备份 Docker volumes 中的数据