# LangLang-Go

> 项目来自 [redlang](https://github.com/super1207/redreply)，为方便想法实施而进行重制。

LangLang-Go 是一个跨平台聊天机器人自定义问答系统，用 Go 语言从零重写。

原项目（RedLang/RedReply）使用 AGPL-3.0 许可，本重制版使用 **BSD 3-Clause** 许可。

原开发者授权说明：
> "开源协议只保护代码，不保护语言设计和思想。"

感谢原作者的创意与开放态度。

---

## 特色

- 支持中文编程（RedLang 脚本引擎）
- 多平台聊天机器人：QQ、Telegram、KOOK、邮件……
- Web UI 可视化脚本管理
- 热重载：编辑脚本无需重启
- 分布式集群（MQTT 5.0）
- 多操作系统：Linux、Windows、FreeBSD、Android

## 构建

```bash
go build ./cmd/langlang
```

## 许可

BSD 3-Clause License. 详见 [LICENSE](LICENSE)。
