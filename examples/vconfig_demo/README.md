# VConfig 示例应用

这个示例展示了如何在实际项目中使用 VConfig 配置模块，包括：

- 创建和管理配置
- 监听配置变更
- 在 HTTP 服务器中使用配置
- 优雅关闭服务器

## 目录结构

```
examples/vconfig_demo/
├── main.go       # 示例应用主程序
├── README.md     # 本文档
└── configs/      # 配置文件目录(运行时创建)
    └── app.yaml  # 应用配置文件(运行时创建)
```

## 运行示例

### 启动示例应用

```bash
# 进入示例目录
cd examples/vconfig_demo

# 运行示例
go run main.go
```

应用启动后，将在控制台输出配置信息，并在本地 8080 端口启动 HTTP 服务器。同时会在当前目录下创建 `configs/app.yaml` 配置文件。

### 访问示例应用

启动后，可以通过浏览器或命令行工具访问：

1. 首页：显示应用基本信息

```bash
curl http://localhost:8080/
```

输出示例：

```
欢迎访问 配置示例应用 (版本: 1.0.0)
环境: development
当前时间: 2023-03-08T12:34:56+08:00
```

2. 配置信息页：显示当前配置

```bash
curl http://localhost:8080/config
```

输出示例：

```
当前配置:
应用名称: 配置示例应用
版本: 1.0.0
环境: development
HTTP端口: 8080
日志级别: info
```

## 测试配置热更新

1. 查看自动生成的配置文件：

```bash
cat configs/app.yaml
```

2. 修改配置文件（如更改应用名称或日志级别）：

```bash
# 使用编辑器修改配置
vi configs/app.yaml

# 或通过命令行直接修改某些值
# Linux/Mac
sed -i 's/level: info/level: debug/' configs/app.yaml

# Windows PowerShell
(Get-Content configs/app.yaml) -replace 'level: info', 'level: debug' | Set-Content configs/app.yaml
```

3. 保存文件后，观察控制台输出，会看到配置变更的日志：

```
配置已更新，重新加载服务器配置
日志级别已更改为: debug
```

4. 再次访问配置信息页，确认配置已更新：

```bash
curl http://localhost:8080/config
```

## 通过环境变量覆盖配置

示例应用支持通过环境变量覆盖配置，环境变量前缀为 `APP_`。例如：

```bash
# 设置环境变量覆盖HTTP端口
# Linux/Mac
export APP_HTTP_PORT=9090

# Windows CMD
set APP_HTTP_PORT=9090

# Windows PowerShell
$env:APP_HTTP_PORT=9090

# 使用新的环境变量启动应用
go run main.go
```

应用将使用环境变量中的端口(9090)而不是配置文件中的端口。

## 关闭应用

按 `Ctrl+C` 可以优雅地关闭应用，应用会正确关闭 HTTP 服务器。

## 部署建议

- 在生产环境中，建议使用环境变量控制应用的关键配置（如数据库连接、API 密钥等）
- 对于非敏感配置，可以使用配置文件，并通过配置热更新来动态调整应用行为
- 配置文件推荐放在应用外部，便于管理和更新
- 对于集群部署，可以考虑使用配置中心作为配置源
