package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequestInfo represents the structure of the request information
type RequestInfo struct {
	FullURL string            `json:"full_url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
}

func main() {
	// Create a new Gin router with default middleware
	r := gin.Default()

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
	fmt.Println("Server is running on http://localhost:8080")
	r.Run(":8080")
}
