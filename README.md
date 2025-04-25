# Prism - HTTP Request Echo Server

Prism 是一个轻量级的 HTTP 请求回显服务器，专门用于测试和调试 webhook 推送功能。它能够接收任何类型的 HTTP 请求，并以格式化的方式显示请求的详细信息。

## 功能特点

- 支持所有 HTTP 方法（GET, POST, PUT, PATCH, DELETE, HEAD 等）
- 自动处理 JSON 请求，格式化显示 JSON 内容
- 支持文件上传，显示文件元信息
- 智能端口管理：
  - 支持通过环境变量配置端口
  - 自动处理端口冲突
  - 自动寻找可用端口
- 跨平台支持：
  - 支持 Windows 和 Linux
  - 支持 AMD64 和 ARM64 架构
- 美观的控制台输出：
  - 彩色显示服务器地址
  - 格式化的请求信息展示
  - 清晰的请求头和请求体显示

## 安装

### 从源码编译

1. 克隆仓库：
```bash
git clone https://github.com/yourusername/prism.git
cd prism
```

2. 编译项目：
```bash
# Windows
.\build.bat

# Linux/Mac
./build.sh
```

编译后的二进制文件将位于 `build` 目录中：
- `prism_windows_amd64.exe` - Windows 64位版本
- `prism_linux_amd64` - Linux 64位版本
- `prism_linux_arm64` - Linux ARM64版本

## 使用方法

### 基本用法

1. 启动服务器：
```bash
# Windows
prism_windows_amd64.exe

# Linux
./prism_linux_amd64
```

2. 服务器将在默认端口（8080）启动，或使用环境变量 `PRISM_PORT` 指定的端口。

### 配置端口

通过环境变量设置端口：

```bash
# Windows
set PRISM_PORT=3000
prism_windows_amd64.exe

# Linux/Mac
export PRISM_PORT=3000
./prism_linux_amd64
```

### 测试示例

1. 发送 GET 请求：
```bash
curl http://localhost:8080/echo
```

2. 发送 JSON 请求：
```bash
curl -X POST http://localhost:8080/echo \
  -H "Content-Type: application/json" \
  -d '{"name": "张三", "age": 25}'
```

3. 上传文件：
```bash
curl -X POST http://localhost:8080/echo \
  -F "file=@/path/to/file.txt" \
  -F "name=test"
```

## 响应格式

服务器返回的 JSON 响应包含以下字段：

```json
{
  "full_url": "http://localhost:8080/echo?param=value",
  "method": "POST",
  "headers": {
    "Content-Type": "application/json",
    "User-Agent": "curl/7.64.1"
  },
  "body": {
    "name": "张三",
    "age": 25
  }
}
```

## 注意事项

1. 服务器会自动处理端口冲突，如果指定端口被占用，会自动寻找下一个可用端口
2. 所有请求信息都会在控制台实时显示
3. JSON 请求会自动格式化显示，方便查看
4. 文件上传时会显示文件元信息，而不是文件内容

## 许可证

MIT License 