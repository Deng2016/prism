package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/term"
)

// RequestInfo represents the structure of the request information
type RequestInfo struct {
	FullURL string            `json:"full_url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
}

// ANSI color codes
const (
	GreenBackground = "\033[42m"
	ResetColor      = "\033[0m"
)

// findAvailablePort tries to find an available port starting from the given port
func findAvailablePort(startPort int) (int, error) {
	port := startPort
	for {
		// Try to create a listener on the port
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			// Port is available
			listener.Close()
			return port, nil
		}

		// Check if the error is due to port being in use
		// Windows error: "Only one usage of each socket address (protocol/network address/port) is normally permitted"
		// Linux error: "address already in use"
		if strings.Contains(strings.ToLower(err.Error()), "address already in use") ||
			strings.Contains(strings.ToLower(err.Error()), "only one usage of each socket address") {
			fmt.Printf("Port %d is in use, trying next port...\n", port)
			port++
			continue
		}

		// Other error occurred
		return 0, err
	}
}

// getPortFromEnv gets the port from environment variable or returns default
func getPortFromEnv() int {
	portStr := os.Getenv("PRISM_PORT")
	if portStr == "" {
		fmt.Println("PRISM_PORT environment variable not set, using default port 8080")
		return 8080
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Printf("Invalid port number in PRISM_PORT: %s, using default port 8080\n", portStr)
		return 8080
	}

	if port <= 0 || port > 65535 {
		fmt.Printf("Port number %d is out of valid range (1-65535), using default port 8080\n", port)
		return 8080
	}

	return port
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

	// Handle all HTTP methods for /echo endpoint
	r.Any("/echo", func(c *gin.Context) {
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

		// Create request info object
		reqInfo := RequestInfo{
			FullURL: c.Request.URL.String(),
			Method:  c.Request.Method,
			Headers: headers,
			Body:    body,
		}

		// Print request details to console
		fmt.Printf("\n=== Request Details ===\n")
		fmt.Printf("URL: %s\n", reqInfo.FullURL)
		fmt.Printf("Method: %s\n", reqInfo.Method)
		fmt.Printf("Headers:\n")
		for key, value := range reqInfo.Headers {
			fmt.Printf("  %s: %s\n", key, value)
		}
		fmt.Printf("Body: %s\n", bodyStr)
		fmt.Printf("=====================\n\n")

		// Return the request info as JSON response
		c.JSON(http.StatusOK, reqInfo)
	})

	// Start the server
	serverAddr := fmt.Sprintf(":%d", port)
	fmt.Printf("Server is running on %shttp://localhost%s%s\n", GreenBackground, serverAddr, ResetColor)
	fmt.Printf("Echo endpoint: %shttp://localhost%s/echo%s\n", GreenBackground, serverAddr, ResetColor)
	if err := r.Run(serverAddr); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}
