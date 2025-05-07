package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
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
	fmt.Printf("\n=== Incoming Request ===\n")
	fmt.Printf("Path: %s\n", c.Request.URL.Path)
	fmt.Printf("Method: %s\n", c.Request.Method)
	fmt.Printf("Remote IP: %s\n", c.Request.RemoteAddr)
	fmt.Printf("=====================\n\n")

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
	fmt.Printf("\n=== Request Details ===\n")
	fmt.Printf("Received Time: %s\n", reqInfo.ReceivedTime)
	fmt.Printf("URL: %s\n", fullURL)
	fmt.Printf("Method: %s\n", reqInfo.Method)
	fmt.Printf("Headers:\n")
	for key, value := range reqInfo.Headers {
		fmt.Printf("  %s: %s\n", key, value)
	}
	fmt.Printf("Body: %s\n", bodyStr)
	fmt.Printf("=====================\n\n")

	// Return the request info as JSON response
	c.JSON(http.StatusOK, reqInfo)
}

func main() {
	// Enable ANSI color support for Windows
	if term.IsTerminal(int(os.Stdout.Fd())) {
		// Enable virtual terminal processing for Windows
		if os.Getenv("TERM") == "" {
			os.Setenv("TERM", "xterm-256color")
		}
	}

	// Get initial port from environment variable
	initialPort := getPortFromEnv()
	fmt.Printf("Initial port: %d\n", initialPort)

	// Find available port
	port, err := findAvailablePort(initialPort)
	if err != nil {
		fmt.Printf("Error finding available port: %v\n", err)
		os.Exit(1)
	}

	if port != initialPort {
		fmt.Printf("Port %d is in use, using port %d instead\n", initialPort, port)
	}

	// Set Gin to release mode
	gin.SetMode(gin.ReleaseMode)

	// Create a new Gin router with default middleware
	r := gin.New()

	// Add debug middleware to log all requests
	r.Use(func(c *gin.Context) {
		fmt.Printf("\n=== Request Debug ===\n")
		fmt.Printf("Incoming request to: %s\n", c.Request.URL.Path)
		fmt.Printf("Method: %s\n", c.Request.Method)
		fmt.Printf("===================\n\n")
		c.Next()
	})

	// Handle all HTTP methods for /webhook and /eventbus endpoints
	r.Any("/webhook", echoHandler)
	r.Any("/eventbus", echoHandler)

	// Start the server with explicit IPv4 binding
	serverAddr := fmt.Sprintf("0.0.0.0:%d", port)
	fmt.Printf("Server is running on %shttp://%s%s\n", GreenBackground, serverAddr, ResetColor)
	fmt.Printf("Webhook endpoint: %shttp://%s/webhook%s\n", GreenBackground, serverAddr, ResetColor)
	fmt.Printf("Eventbus endpoint: %shttp://%s/eventbus%s\n", GreenBackground, serverAddr, ResetColor)

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
		fmt.Printf("Error creating listener: %v\n", err)
		os.Exit(1)
	}

	server := &http.Server{
		Handler: r,
	}

	// Start the server
	if err := server.Serve(listener); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}
