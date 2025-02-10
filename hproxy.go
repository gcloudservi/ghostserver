package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv" // 确保导入 strconv 包
	"strings"
)

// 全局变量用于配置目录路径，方便修改
var (
	versionsDir = "versions" // 版本文件目录
	routeDir    = "route"    // 路由文件目录
	httpDir     = "http"     // HTTP 文件目录
	port        = ":8089"    // 监听端口，修改为 8089
	errorFile   = "http/error.json" // 错误信息文件路径
	redirectURL = "https://update.version.brmyx.com/dmm_share/index.html?bundle=com.bairimeng.dmmdzz.betazone" // 跳转 URL
)

// ErrorResponse 定义错误响应结构
type ErrorResponse struct {
	ErrorCode int    `json:"errorCode"`
	ErrorMsg  string `json:"errorMsg,omitempty"` // 错误信息，omitempty 表示为空时不输出
}

func main() {
	// 启动 HTTP 服务器
	http.HandleFunc("/", requestHandler)
	log.Printf("服务器已启动，监听端口 %s", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("服务器启动失败: ", err)
	}
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("接收到请求: 方法=%s, 路径=%s", r.Method, r.URL.Path)
	//log.Printf("完整URL: %s", r.URL.String())
	//log.Printf("请求头: %+v", r.Header)
	//contentType := r.Header.Get("Content-Type")
	//log.Printf("Content-Type: %s", contentType)

	// 获取程序运行目录
	programDir, err := os.Getwd()
	if err != nil {
		log.Println("获取程序目录失败:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 处理 GET 请求
	if r.Method == http.MethodGet {
		if strings.HasPrefix(r.URL.Path, "/hide/version/") || strings.HasPrefix(r.URL.Path, "/hide/versions/") {
			// 转发到 versions 目录
			filePath := filepath.Join(programDir, versionsDir, filepath.Base(r.URL.Path))
			log.Printf("转发 GET 请求到文件: %s", filePath)
			serveFile(filePath, w, r) // 使用正确的 serveFile 调用
			return
		} else if strings.HasPrefix(r.URL.Path, "/hide/") {
			// 转发到 route 目录
			filePath := filepath.Join(programDir, routeDir, filepath.Base(r.URL.Path))
			log.Printf("转发 GET 请求到文件: %s", filePath)
			serveFile(filePath, w, r) // 使用正确的 serveFile 调用
			return
		} else {
			// 其他 GET 请求，执行 302 跳转
			//log.Printf("GET 请求路径未匹配，跳转到: %s", redirectURL)
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}
	} else if r.Method == http.MethodPost {
		// 处理 POST 请求
		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("读取 POST 请求体失败:", err)
			errorResponse(w, programDir, "读取请求体失败", err) // 使用 errorResponse 返回错误
			return
		}
		bodyString := string(bodyBytes)

		// 手动解析 text/plain 请求体
		msgID, msg, err := parseTextPlainBody(bodyString)
		if err != nil {
			log.Println("解析 POST 请求体失败:", err)
			errorResponse(w, programDir, "解析 POST 请求体失败", err) // 使用 errorResponse 返回错误
			return
		}

		log.Printf("接收到 POST 请求, msg_id=%d, msg=%s", msgID, msg)

		// 构建 JSON 文件路径
		filePath := filepath.Join(programDir, httpDir, fmt.Sprintf("%d.json", msgID))
		log.Printf("转发 POST 请求到文件: %s", filePath)

		// 检查文件是否存在
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Printf("文件不存在: %s", filePath)
			errorResponse(w, programDir, "文件不存在", fmt.Errorf("file not found: %s", filePath)) // 使用 errorResponse 返回错误
			return
		}

		// 读取 JSON 文件内容
		jsonContent, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Printf("读取文件 %s 失败: %v", filePath, err)
			errorResponse(w, programDir, "读取文件失败", err) // 使用 errorResponse 返回错误
			return
		}

		// 设置响应头并返回 JSON 内容
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonContent)
		return
	}

	// 其他情况返回 404
	log.Println("未匹配到任何转发规则，返回 404")
	http.NotFound(w, r)
}

// errorResponse 读取 error.json 文件并返回错误响应
func errorResponse(w http.ResponseWriter, programDir string, logMsg string, err error) {
	log.Println(logMsg, ":", err) // 记录日志

	errorFilePath := filepath.Join(programDir, errorFile)
	errorContent, readErr := ioutil.ReadFile(errorFilePath)
	if readErr != nil {
		log.Println("读取错误信息文件失败:", readErr)
		// 如果读取 error.json 失败，则返回默认错误响应
		defaultErrorResponse := ErrorResponse{ErrorCode: -1, ErrorMsg: "Internal Server Error"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // 仍然返回 200 OK
		json.NewEncoder(w).Encode(defaultErrorResponse)
		return
	}

	var errResp ErrorResponse
	if jsonErr := json.Unmarshal(errorContent, &errResp); jsonErr != nil {
		log.Println("解析错误信息 JSON 失败:", jsonErr)
		// 如果解析 error.json 失败，则返回默认错误响应
		defaultErrorResponse := ErrorResponse{ErrorCode: -1, ErrorMsg: "Internal Server Error"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // 仍然返回 200 OK
		json.NewEncoder(w).Encode(defaultErrorResponse)
		return
	}

	// 添加 ErrorMsg
	errResp.ErrorMsg = err.Error()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // 仍然返回 200 OK
	json.NewEncoder(w).Encode(errResp)
}


// parseTextPlainBody 用于解析 text/plain 类型的请求体
func parseTextPlainBody(body string) (int, string, error) {
	msgID := -1
	msg := ""

	pairs := strings.Split(body, "&")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue // 忽略格式错误的键值对
		}
		key := parts[0]
		value := parts[1]

		switch key {
		case "msg_id":
			id, err := strconv.Atoi(value)
			if err != nil {
				return -1, "", fmt.Errorf("invalid msg_id: %w", err)
			}
			msgID = id
		case "msg":
			msg = value
		}
	}

	if msgID == -1 {
		return -1, "", fmt.Errorf("missing msg_id")
	}

	return msgID, msg, nil
}

// 修改 serveFile 函数，添加 w http.ResponseWriter, r *http.Request 参数
// 正确的 serveFile 函数定义
func serveFile(filePath string, w http.ResponseWriter, r *http.Request) {
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("文件不存在: %s", filePath)
		// 文件不存在时，执行 302 跳转
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// 转发文件
	http.ServeFile(w, r, filePath)
	log.Printf("成功转发文件: %s", filePath)
}
