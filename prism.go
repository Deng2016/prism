package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/term"
)

// RequestInfo represents the structure of the request information
type RequestInfo struct {
	FullURL      string            `json:"full_url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers"`
	Body         interface{}       `json:"body"`
	ReceivedTime string            `json:"received_time"`
}

// ANSI color codes
const (
	GreenBackground = "\033[42m"
	ResetColor      = "\033[0m"
)

// 全局日志文件
var logFile *os.File

// 初始化日志文件
func initLogFile() error {
	// 获取当前可执行文件所在目录
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}
	exeDir := filepath.Dir(exePath)

	// 创建日志文件
	logPath := filepath.Join(exeDir, "prism.log")
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %v", err)
	}

	// 设置日志输出到文件和控制台
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	// 打印日志文件路径
	fmt.Printf("日志文件保存路径: %s\n", logPath)

	return nil
}

// 自定义的日志输出函数
func logPrint(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	log.Print(message)
}

// findAvailablePort tries to find an available port starting from the given port
func findAvailablePort(startPort int) (int, error) {
	port := startPort
	maxPort := 65535
	maxAttempts := 100 // Limit the number of attempts to avoid infinite loops

	attempts := 0
	for port <= maxPort && attempts < maxAttempts {
		attempts++

		// Create a TCP listener with specific options
		addr := fmt.Sprintf(":%d", port)
		config := net.ListenConfig{
			Control: func(network, address string, c syscall.RawConn) error {
				return c.Control(func(fd uintptr) {
					// On Windows, use both SO_REUSEADDR and SO_EXCLUSIVEADDRUSE
					if runtime.GOOS == "windows" {
						// First set SO_REUSEADDR to 0
						syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0)
						// Then set SO_EXCLUSIVEADDRUSE
						syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, 0x0000000F, 1)
					} else {
						// On other systems, use SO_REUSEADDR
						syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0)
					}
				})
			},
		}

		// Try to create a listener with the specific configuration
		listener, err := config.Listen(context.Background(), "tcp4", addr)
		if err == nil {
			// Port is available
			listener.Close()
			return port, nil
		}

		// Check if the error is due to port being in use or permission issues
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "address already in use") ||
			strings.Contains(errStr, "only one usage of each socket address") ||
			strings.Contains(errStr, "access permissions") {
			fmt.Printf("Port %d is not available (%s), trying next port...\n", port, errStr)
			port++
			continue
		}

		// Other error occurred
		return 0, fmt.Errorf("error checking port %d: %v", port, err)
	}

	return 0, fmt.Errorf("no available ports found after %d attempts", maxAttempts)
}

// getPortFromEnv gets the port from environment variable or returns default
func getPortFromEnv() int {
	portStr := os.Getenv("PRISM_PORT")
	if portStr == "" {
		fmt.Println("PRISM_PORT environment variable not set, using default port 38438")
		return 38438
	}

	// Trim whitespace and any non-numeric characters
	portStr = strings.TrimSpace(portStr)
	// Extract only the numeric part
	for i, c := range portStr {
		if c < '0' || c > '9' {
			portStr = portStr[:i]
			break
		}
	}

	if portStr == "" {
		fmt.Printf("Invalid port number in PRISM_PORT: %s, using default port 38438\n", os.Getenv("PRISM_PORT"))
		return 38438
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Printf("Invalid port number in PRISM_PORT: %s, using default port 38438\n", os.Getenv("PRISM_PORT"))
		return 38438
	}

	if port <= 0 || port > 65535 {
		fmt.Printf("Port number %d is out of valid range (1-65535), using default port 38438\n", port)
		return 38438
	}

	return port
}

// getFullURL constructs the full URL including scheme, host, and path
func getFullURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}

	host := c.Request.Host
	if host == "" {
		host = c.Request.RemoteAddr
	}

	return fmt.Sprintf("%s://%s%s", scheme, host, c.Request.URL.String())
}

// echoHandler handles the request and returns the request information
func echoHandler(c *gin.Context) {
	// Print debug information about the request
	logPrint("\n=== Incoming Request ===")
	logPrint("Path: %s", c.Request.URL.Path)
	logPrint("Method: %s", c.Request.Method)
	logPrint("Remote IP: %s", c.Request.RemoteAddr)
	logPrint("=====================\n")

	// Create headers map
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		headers[key] = strings.Join(values, ", ")
	}

	var body interface{}
	var bodyStr string

	// Check if the request contains files
	if c.Request.MultipartForm != nil && c.Request.MultipartForm.File != nil {
		// Handle multipart form with files
		formData := make(map[string]interface{})

		// Process form fields
		for key, values := range c.Request.MultipartForm.Value {
			if len(values) == 1 {
				formData[key] = values[0]
			} else {
				formData[key] = values
			}
		}

		// Process files
		for key, files := range c.Request.MultipartForm.File {
			fileInfos := make([]map[string]interface{}, len(files))
			for i, file := range files {
				fileInfos[i] = map[string]interface{}{
					"filename": file.Filename,
					"size":     file.Size,
					"header":   file.Header,
				}
			}
			formData[key] = fileInfos
		}
		body = formData
		bodyStr = "[Multipart Form Data with Files]"
	} else {
		// Handle regular request body
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			// Restore the request body for subsequent reads
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Check if the request is JSON
		contentType := c.GetHeader("Content-Type")
		if strings.Contains(contentType, "application/json") && len(bodyBytes) > 0 {
			// Try to parse JSON
			var jsonBody interface{}
			if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
				body = jsonBody
				// Format JSON for console output
				prettyJSON, _ := json.MarshalIndent(jsonBody, "", "  ")
				bodyStr = string(prettyJSON)
			} else {
				body = string(bodyBytes)
				bodyStr = string(bodyBytes)
			}
		} else {
			body = string(bodyBytes)
			bodyStr = string(bodyBytes)
		}
	}

	// Get full URL including scheme and host
	fullURL := getFullURL(c)

	// Create request info object
	reqInfo := RequestInfo{
		FullURL:      fullURL,
		Method:       c.Request.Method,
		Headers:      headers,
		Body:         body,
		ReceivedTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Print request details to console
	logPrint("\n=== Request Details ===")
	logPrint("Received Time: %s", reqInfo.ReceivedTime)
	logPrint("URL: %s", fullURL)
	logPrint("Method: %s", reqInfo.Method)
	logPrint("Headers:")
	for key, value := range reqInfo.Headers {
		logPrint("  %s: %s", key, value)
	}
	logPrint("Body: %s", bodyStr)
	logPrint("=====================\n")

	// Return the request info as JSON response
	c.JSON(http.StatusOK, reqInfo)
}

func main() {
	// 初始化日志文件
	if err := initLogFile(); err != nil {
		fmt.Printf("初始化日志文件失败: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	// Enable ANSI color support for Windows
	if term.IsTerminal(int(os.Stdout.Fd())) {
		// Enable virtual terminal processing for Windows
		if os.Getenv("TERM") == "" {
			os.Setenv("TERM", "xterm-256color")
		}
	}

	// Get initial port from environment variable
	initialPort := getPortFromEnv()
	logPrint("Initial port: %d", initialPort)

	// Find available port
	port, err := findAvailablePort(initialPort)
	if err != nil {
		logPrint("Error finding available port: %v", err)
		os.Exit(1)
	}

	if port != initialPort {
		logPrint("Port %d is in use, using port %d instead", initialPort, port)
	}

	// Set Gin to release mode
	gin.SetMode(gin.ReleaseMode)

	// Create a new Gin router with default middleware
	r := gin.New()

	// Add debug middleware to log all requests
	r.Use(func(c *gin.Context) {
		logPrint("\n=== Request Debug ===")
		logPrint("Incoming request to: %s", c.Request.URL.Path)
		logPrint("Method: %s", c.Request.Method)
		logPrint("===================\n")
		c.Next()
	})

	// Handle all HTTP methods for /webhook and /eventbus endpoints
	r.Any("/webhook", echoHandler)
	r.Any("/eventbus", echoHandler)

	// Start the server with explicit IPv4 binding
	serverAddr := fmt.Sprintf("0.0.0.0:%d", port)
	logPrint("Server is running on %shttp://%s%s", GreenBackground, serverAddr, ResetColor)
	logPrint("Webhook endpoint: %shttp://%s/webhook%s", GreenBackground, serverAddr, ResetColor)
	logPrint("Eventbus endpoint: %shttp://%s/eventbus%s", GreenBackground, serverAddr, ResetColor)

	// Create a custom server with IPv4-only configuration and strict port binding
	config := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// On Windows, use both SO_REUSEADDR and SO_EXCLUSIVEADDRUSE
				if runtime.GOOS == "windows" {
					// First set SO_REUSEADDR to 0
					syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0)
					// Then set SO_EXCLUSIVEADDRUSE
					syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, 0x0000000F, 1)
				} else {
					// On other systems, use SO_REUSEADDR
					syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0)
				}
			})
		},
	}

	listener, err := config.Listen(context.Background(), "tcp4", serverAddr)
	if err != nil {
		logPrint("Error creating listener: %v", err)
		os.Exit(1)
	}

	server := &http.Server{
		Handler: r,
	}

	// Start the server
	if err := server.Serve(listener); err != nil {
		logPrint("Error starting server: %v", err)
		os.Exit(1)
	}
}
