# WLBaiSH - Wang-Laoban AI Shell 🐚🤖

WLBaiSH（Wang-Laoban AI Shell）是一个由大模型驱动的终端操作助手。它通过自然语言对话，自动生成并执行 Linux Shell 命令，实现系统维护、软件安装、配置修改等日常运维任务。

## ✨ 项目特色

- 💬 基于自然语言与大模型对话（支持 DeepSeek 等 LLM 接口）
- 🖥️ 可视化 Web 前端（Gin + Bootstrap）
- ⚙️ 支持 Linux 系统命令执行
- 🔐 SSH 信息本地配置，安全可控
- 🔍 实时调试输出，命令执行过程透明可追踪

## 🖼️ 项目结构

```
.
├── main.go               # 核心后端逻辑
├── templates/
│   └── index.html        # Web 前端页面
├── static/               # 静态资源目录（可选）
├── go.mod                # Go module 管理
└── README.md             # 项目说明
```

## 🚀 快速启动

### 1. 环境准备

- Go 1.18+
- 已注册并获取 [DeepSeek API Key](https://deepseek.com)

### 2. 设置环境变量

```bash
export DEEPSEEK_API_KEY=your-api-key-here
```

### 3. 运行项目

```bash
go run main.go
```

访问页面：[http://localhost:8080](http://localhost:8080)

## 🧪 使用示例

在 Web 页面左侧输入：
```
请帮我查询下系统是 ubuntu 还是 centos
```

右侧将显示：
- 模型返回的原始回复
- 提取的 Shell 命令（如：`cat /etc/os-release | grep PRETTY_NAME`）
- 实际执行结果（如：`PRETTY_NAME="Ubuntu 20.04.6 LTS"`）

## 🛡️ 安全说明

- 本地仅允许访问预设的 SSH 终端或本地环境，请勿直接开放公网使用。
- 建议结合操作确认、命令审核、白名单控制等机制用于生产环境。

## 🗺️ 未来计划

- ✅ 多轮对话支持
- ✅ 交互式命令确认机制
- ⏳ 命令执行日志审计
- ⏳ 多主机 SSH 管理
- ⏳ 指令集白名单安全限制

## 🤝 致谢

本项目灵感来源于 AI 与自动化结合的场景，感谢 DeepSeek 提供的高质量 API。

---

> 🧠 项目名称含义：**WLBaiSH = Wang-Laoban AI Shell**，寓意“老板指令一句话，Shell 全搞定。”
