# LangLang-Go

> 项目来自 [redlang](https://github.com/super1207/redreply)，为方便想法实施而进行重制。

LangLang-Go 是一个跨平台聊天机器人自定义问答系统，用 Go 语言从零重写。

原项目（RedLang/RedReply）使用 AGPL-3.0 许可，本重制版使用 **BSD 3-Clause** 许可。

redlang原开发者授权说明：
> "开源协议只保护代码，不保护语言设计和思想。"

感谢原作者的创意与开放态度。

---

## 特色

- **中文编程** — 内置 RedLang 脚本引擎，支持中文脚本语法
- **多平台支持** — QQ（OneBot 11）、Telegram、KOOK、邮件……
- **Web UI 管理** — 内嵌静态资源，浏览器直接访问即可管理脚本
- **热重载** — 编辑脚本无需重启，即时生效
- **分布式集群** — 基于 MQTT 5.0 的集群通信
- **跨平台** — 支持 Linux、Windows、FreeBSD、Android

## 构建

```bash
go build ./cmd/langlang
```

构建后直接运行生成的可执行文件，Web UI 默认监听 `http://localhost:8080`。

## 项目结构

```
langlang-go/
├── cmd/langlang/          # 入口
├── internal/
│   ├── bot/               # 机器人适配器（QQ、Telegram 等）
│   ├── config/            # 配置管理
│   ├── redlang/           # RedLang 脚本引擎
│   ├── webui/             # Web UI 服务器（内嵌静态资源）
│   ├── mqtt/              # MQTT 集群客户端
│   ├── db/                # 数据库
│   ├── plugin/            # 插件管理器
│   └── ...                # 其他工具模块
├── web/                   # 旧版 Web UI（已弃用）
└── config.default.json    # 默认配置模板
```

## 许可

BSD 3-Clause License. 详见 [LICENSE](LICENSE)。