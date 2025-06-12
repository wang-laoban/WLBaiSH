package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
)

type ChatRequest struct {
	Message string `json:"message"`
}

type ShellResult struct {
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	Command   string `json:"command"`
	LLMOutput string `json:"llm_output"`
}

type DeepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepSeekPayload struct {
	Model    string            `json:"model"`
	Messages []DeepSeekMessage `json:"messages"`
	Stream   bool              `json:"stream"`
}

type DeepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func callLLMAPI(message string) (string, string, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return "", "", fmt.Errorf("DeepSeek API Key not set in environment variable DEEPSEEK_API_KEY")
	}

	payload := DeepSeekPayload{
		Model:  "deepseek-chat",
		Stream: false,
		Messages: []DeepSeekMessage{
			{Role: "system", Content: "你是一个Linux运维专家，只返回可执行的Shell命令，避免解释内容、示例命令、换行注释。"},
			{Role: "user", Content: message},
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequest("POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("LLM API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var dsResp DeepSeekResponse
	if err := json.NewDecoder(resp.Body).Decode(&dsResp); err != nil {
		return "", "", err
	}

	if len(dsResp.Choices) == 0 {
		return "", "", fmt.Errorf("No choices returned from DeepSeek")
	}

	content := dsResp.Choices[0].Message.Content
	command := extractFirstCommand(content)
	return command, content, nil
}

func extractFirstCommand(response string) string {
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return response
}

func executeShellCommand(cmdStr string) (string, string) {
	cmd := exec.Command("bash", "-c", cmdStr)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return outBuf.String(), errBuf.String()
	}
	return outBuf.String(), ""
}

func main() {
	r := gin.Default()
	r.Static("/static", "./static")
	r.LoadHTMLFiles("templates/index.html")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.POST("/chat", func(c *gin.Context) {
		var req ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		cmd, llmRaw, err := callLLMAPI(req.Message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		out, errOut := executeShellCommand(cmd)
		c.JSON(http.StatusOK, ShellResult{
			Output:    out,
			Error:     errOut,
			Command:   cmd,
			LLMOutput: llmRaw,
		})
	})

	r.Run(":8080")
}

