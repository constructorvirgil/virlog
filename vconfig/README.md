# VConfig - 基于 Viper 的通用配置管理模块

VConfig 是一个基于[Viper](https://github.com/spf13/viper)的通用配置管理模块，通过使用 Go 1.18+的泛型特性实现，提供了简单易用的配置管理功能。

## 特性

- 🧩 **泛型支持**：使用 Go 泛型，只需定义配置结构体即可
- 🔄 **多来源配置**：同时支持配置文件和环境变量
- 👀 **配置监控**：自动监控配置文件变更并重新加载
- 🔔 **变更通知**：提供配置变更的回调机制
- 🛡️ **类型安全**：完全类型安全的配置访问
- 🧠 **智能默认值**：支持默认配置值
- ⏱️ **防抖处理**：配置文件变更时的防抖处理

## 安装

```bash
go get github.com/virlog/vconfig
```

## 快速开始

1. 定义配置结构体：

```go
type AppConfig struct {
    App struct {
        Name    string `yaml:"name"`
        Version string `yaml:"version"`
    } `yaml:"app"`
    Server struct {
        Host string `yaml:"host"`
        Port int    `yaml:"port"`
    } `yaml:"server"`
}
```

2. 创建默认配置：

```go
func newDefaultConfig() AppConfig {
    config := AppConfig{}
    config.App.Name = "我的应用"
    config.App.Version = "1.0.0"
    config.Server.Host = "localhost"
    config.Server.Port = 8080
    return config
}
```

3. 初始化配置管理器：

```go
// 创建配置实例
cfg, err := vconfig.NewConfig(newDefaultConfig(),
    vconfig.WithConfigFile[AppConfig]("config.yaml"),
    vconfig.WithEnvPrefix[AppConfig]("APP"))

if err != nil {
    log.Fatalf("初始化配置失败: %v", err)
}
```

4. 使用配置：

```go
// 获取配置
config := cfg.Get()
fmt.Println("应用名称:", config.App.Name)
fmt.Println("服务器端口:", config.Server.Port)
```

5. 监听配置变更：

```go
// 添加配置变更回调
cfg.OnChange(func(e fsnotify.Event) {
    fmt.Println("配置已更新，需要重新加载某些组件")
    // 获取最新配置
    newConfig := cfg.Get()
    // 执行相应操作...
})
```

## 高级用法

### 环境变量覆盖

VConfig 支持使用环境变量覆盖配置文件中的值。环境变量的命名规则为：`前缀_结构体字段_嵌套字段`，字段之间使用`_`连接，全部大写。

例如，对于以下配置结构体：

```go
type AppConfig struct {
    Server struct {
        Port int `yaml:"port"`
    } `yaml:"server"`
}
```

可以使用环境变量`APP_SERVER_PORT=9000`来覆盖配置文件中的`server.port`值。

### 配置文件类型

VConfig 支持多种配置文件类型，包括 YAML、JSON 和 TOML。默认使用 YAML 格式，可以通过`WithConfigType`选项更改：

```go
cfg, err := vconfig.NewConfig(defaultConfig,
    vconfig.WithConfigFile[AppConfig]("config.json"),
    vconfig.WithConfigType[AppConfig](vconfig.JSON))
```

### 保存配置

可以通过`SaveConfig`方法将配置保存到文件：

```go
// 更新配置
cfg.Data.Server.Port = 9000

// 保存配置
err := cfg.SaveConfig()
if err != nil {
    log.Fatalf("保存配置失败: %v", err)
}
```

也可以使用`Update`方法一次性更新并保存配置：

```go
newConfig := AppConfig{}
// 设置新的配置值...

err := cfg.Update(newConfig)
if err != nil {
    log.Fatalf("更新配置失败: %v", err)
}
```

### 防抖设置

为了避免配置文件频繁变更导致过多的回调触发，VConfig 内置了防抖机制，默认防抖时间为 500 毫秒。可以通过`WithDebounceTime`选项修改：

```go
cfg, err := vconfig.NewConfig(defaultConfig,
    vconfig.WithConfigFile[AppConfig]("config.yaml"),
    vconfig.WithDebounceTime[AppConfig](time.Second))
```

## 完整示例

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/fsnotify/fsnotify"
    "github.com/virlog/vconfig"
)

// 定义配置结构体
type AppConfig struct {
    App struct {
        Name    string `yaml:"name"`
        Version string `yaml:"version"`
    } `yaml:"app"`
    Server struct {
        Host string `yaml:"host"`
        Port int    `yaml:"port"`
    } `yaml:"server"`
    Database struct {
        DSN      string `yaml:"dsn"`
        MaxConns int    `yaml:"max_conns"`
    } `yaml:"database"`
}

// 创建默认配置
func newDefaultConfig() AppConfig {
    config := AppConfig{}
    config.App.Name = "示例应用"
    config.App.Version = "1.0.0"
    config.Server.Host = "localhost"
    config.Server.Port = 8080
    config.Database.DSN = "postgres://user:password@localhost:5432/dbname"
    config.Database.MaxConns = 10
    return config
}

func main() {
    // 创建配置实例
    cfg, err := vconfig.NewConfig(newDefaultConfig(),
        vconfig.WithConfigFile[AppConfig]("config.yaml"),
        vconfig.WithEnvPrefix[AppConfig]("APP"))

    if err != nil {
        log.Fatalf("初始化配置失败: %v", err)
    }

    // 添加配置变更回调
    cfg.OnChange(func(e fsnotify.Event) {
        fmt.Println("配置已更新:", e.Name)
        config := cfg.Get()
        fmt.Printf("新配置: %+v\n", config)
    })

    // 打印初始配置
    config := cfg.Get()
    fmt.Printf("初始配置: %+v\n", config)

    // 程序运行，等待配置文件变更
    fmt.Println("程序运行中，可以修改配置文件", cfg.GetViper().ConfigFileUsed())
    select {}
}
```

## 许可证

MIT
