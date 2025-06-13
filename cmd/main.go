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
	AutoRun bool   `json:"auto_run"`
}

type ShellResult struct {
	Output    string   `json:"output"`
	Error     string   `json:"error,omitempty"`
	Command   string   `json:"command"`
	LLMOutput string   `json:"llm_output"`
	Queue     []string `json:"queue,omitempty"`
	Hint      string   `json:"hint,omitempty"`
	Log       string   `json:"log,omitempty"`
	Status    string   `json:"status,omitempty"`
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
	fmt.Println("正在调用DeepSeek LLM API，消息内容：", message)
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
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "[Hint]") {
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
	_ = cmd.Run()
	return outBuf.String(), errBuf.String()
}

func diagnoseAndSuggest(originalCommand, stderr string) ([]string, string) {
	fmt.Println("已向LLM发送诊断请求，命令：", originalCommand)
	prompt := fmt.Sprintf(`你是一个专业的Linux运维助手。我刚刚执行的命令是：
%s

其错误输出是：
%s

请你：
1. 判断是否需要执行一些前置命令（如 docker ps）以完成用户意图；
2. 如果需要，请返回一个完整的命令队列，每一行一个命令，最后一条必须是原始命令；
3. 如果你认为应该让我确认容器名、路径、服务名等，请追加一句简短确认提示，以“请确认”或“是否”为开头；
4. 不要添加任何解释，不要换行注释，仅返回命令和提示。`, originalCommand, stderr)

	suggestion, _, err := callLLMAPI(prompt)
	if err != nil {
		return nil, ""
	}

	tasks := []string{}
	confirmHint := ""
	lines := strings.Split(suggestion, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[Hint]") {
			confirmHint = strings.TrimPrefix(line, "[Hint] ")
		} else {
			tasks = append(tasks, line)
		}
	}
	fmt.Println("LLM已返回建议的命令队列：", tasks)

	return tasks, confirmHint
}

func verifyCompletion(message string, output string) (string, string) {
	prompt := fmt.Sprintf("用户意图是：%s\n命令输出结果是：\n%s\n\n请判断用户目标是否已完成？已完成只回复已完成， 未完成回复未完成以及原因。若未完成，请简要说明原因。", message, output)
	reply, _, err := callLLMAPI(prompt)
	if err != nil {
		return "", "无法判断目标完成状态：" + err.Error()
	}
	summary := strings.TrimSpace(reply)
	if strings.Contains(summary, "未完成") {
		return "未完成", summary
	}
	return "已完成", summary
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
		queue := []string{cmd}
		confirmHint := ""
		logBuffer := fmt.Sprintf("用户输入：%s\nLLM建议命令：%s\nLLM原始输出：%s\n", req.Message, cmd, llmRaw)
		status := ""

		if errOut != "" {
			queue, confirmHint = diagnoseAndSuggest(cmd, errOut)
			logBuffer += fmt.Sprintf("Shell错误：%s\n诊断命令队列：%v\n提示：%s\n", errOut, queue, confirmHint)

			if req.AutoRun {
				out = ""
				errOut = ""
				for _, q := range queue {
					o, e := executeShellCommand(q)
					out += fmt.Sprintf("[执行命令]: %s\n[输出结果]:\n%s\n", q, o)
					errOut += e
				}
				logBuffer += fmt.Sprintf("自动执行命令输出：%s\n", out)
			}
		}

		status, statusMsg := verifyCompletion(req.Message, out)
		logBuffer += fmt.Sprintf("意图理解与验证：%s\n", statusMsg)

		fmt.Println("==== Chat Session ====")
		fmt.Println(logBuffer)
		fmt.Println("Shell返回（stdout）：", out)
		fmt.Println("Shell返回（stderr）：", errOut)
		fmt.Println("======================")

		c.JSON(http.StatusOK, ShellResult{
			Output:    out,
			Error:     errOut,
			Command:   cmd,
			LLMOutput: llmRaw,
			Queue:     queue,
			Hint:      confirmHint,
			Log:       logBuffer,
			Status:    status,
		})
	})

	r.Run(":8080")
}
