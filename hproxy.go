package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings" // 导入 strings 包
	"time"
)

const (
	port         = 8089
	httpDir      = "http" // 存放 HTTP 文件的子目录
	routeXMLPath = "hide/route.xml"
	redirectURL  = "https://update.version.brmyx.com/dmm_share/index.html?bundle=com.bairimeng.dmmdzz.betazone"
)

func main() {
	// 设置日志同时输出到文件和控制台
	logFile, err := os.OpenFile("server.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("无法打开日志文件: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.LstdFlags | log.Lshortfile) // 日志中包含时间戳和文件:行号

	// 如果 http 目录不存在，则创建它
	if _, err := os.Stat(httpDir); os.IsNotExist(err) {
		if err := os.Mkdir(httpDir, 0755); err != nil {
			log.Fatalf("创建 http 目录失败: %v", err)
		}
	}

	// HTTP 请求处理函数
	http.HandleFunc("/", requestHandler)

	// 启动服务器
	addr := fmt.Sprintf(":%d", port)
	log.Printf("服务器已启动，端口号 %d", port)
	log.Printf("针对 /%s 的请求将返回 route.xml", routeXMLPath)
	log.Printf("针对 /hide/versions/ 的请求将返回 versions 目录下的文件")
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("ListenAndServe 错误: %v", err)
	}
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	log.Printf("收到请求: %s %s 来自 %s", r.Method, r.URL, r.RemoteAddr)

	// 检查请求路径是否匹配特定路径
	if strings.HasPrefix(r.URL.Path, "/hide/route.") && r.Method == "GET" { // 修改为前缀判断
		serveRouteXML(w, r)
	} else {
		// 处理 POST 请求
		if r.Method == "POST" {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				log.Printf("读取请求体失败: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(fmt.Sprintf(`{"errorCode":-2,"errorMsg":"Failed to read request body: %v"}`, err)))
				return
			}
			defer r.Body.Close()

			// 解析 POST 数据
			values, err := url.ParseQuery(string(body))
			if err != nil {
				log.Printf("解析 POST 数据失败: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(fmt.Sprintf(`{"errorCode":-3,"errorMsg":"Failed to parse POST data: %v"}`, err)))
				return
			}

			// 获取并解码 msg 参数
			msgEncoded := values.Get("msg")
			msgDecoded, err := url.QueryUnescape(msgEncoded) // 进行 URL 解码
			// 输出解码后的 msg 数据
			log.Printf("收到 POST 请求体数据: msg_id=%s&msg=%s", values.Get("msg_id"), msgDecoded)

			msgID := values.Get("msg_id")
			if msgID == "" {
				log.Printf("未找到 msg_id 参数")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"errorCode":-9,"errorMsg":"msg_id parameter not found"}`))
				return
			}

			// 根据 msg_id 构建 JSON 文件路径
			jsonFilePath := filepath.Join(httpDir, msgID+".json")

			// 检查 JSON 文件是否存在
			if _, err := os.Stat(jsonFilePath); os.IsNotExist(err) {
				log.Printf("文件 %s 不存在", jsonFilePath)
				// 返回自定义的错误信息
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK) //这里可以改为其他的状态码
				w.Write([]byte(fmt.Sprintf(`{"errorCode":-13,"errorMsg":"File %s not found"}`, msgID)))
				return
			}

			// 读取 JSON 文件内容
			jsonContent, err := ioutil.ReadFile(jsonFilePath)
			if err != nil {
				log.Printf("读取文件 %s 失败: %v", jsonFilePath, err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// 设置响应头并返回 JSON 内容
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonContent)

			log.Printf("成功返回 %s 给 %s", jsonFilePath, r.RemoteAddr)

		} else if strings.HasPrefix(r.URL.Path, "/hide/versions/") && r.Method == "GET" {
			serveVersions(w, r)
		} else if strings.HasPrefix(r.URL.Path, "/hide/version/") && r.Method == "GET" { // 添加对 /hide/version/ 路径的处理
			serveVersions(w, r)
		} else {
			http.Redirect(w, r, redirectURL, http.StatusFound)
		}
	}

	duration := time.Since(startTime)
	log.Printf("请求处理耗时 %v", duration)
}

func serveRouteXML(w http.ResponseWriter, r *http.Request) {
	// 从请求的 URL 中提取文件名
	u, err := url.Parse(r.URL.String())
	if err != nil {
		log.Printf("解析 URL 失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"errorCode":-7,"errorMsg":"Failed to parse URL: %v"}`, err)))
		return
	}
	path := u.Path
	filename := filepath.Base(path) // 提取文件名，例如 route.xml 或 fyuyu.xml

	// 构建 route 目录下的文件的完整路径
	routeDir := filepath.Join(httpDir, "route") // route 目录
	xmlFilePath := filepath.Join(routeDir, filename) // 完整路径

	log.Printf("route.xml 文件路径: %s", xmlFilePath) // 更新日志

	// 检查 route.xml 是否存在
	if _, err := os.Stat(xmlFilePath); os.IsNotExist(err) {
		log.Printf("route.xml 文件不存在: %s", xmlFilePath)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"errorCode":-5,"errorMsg":"route.xml file not found: %s"}`, xmlFilePath)))
		return
	}

	// 读取 route.xml 的内容
	xmlContent, err := ioutil.ReadFile(xmlFilePath)
	if err != nil {
		log.Printf("读取 route.xml 文件失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"errorCode":-6,"errorMsg":"Failed to read route.xml file: %v"}`, err)))
		return
	}

	// 设置响应头并发送 XML 内容
	w.Header().Set("Content-Type", "application/xml")
	w.Write(xmlContent)

	log.Printf("成功返回 route.xml 给 %s", r.RemoteAddr)
}

func serveVersions(w http.ResponseWriter, r *http.Request) {
	// 从请求的 URL 中提取文件名
	u, err := url.Parse(r.URL.String())
	if err != nil {
		log.Printf("解析 URL 失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"errorCode":-7,"errorMsg":"Failed to parse URL: %v"}`, err)))
		return
	}
	path := u.Path
	filename := filepath.Base(path)

	// 构建 versions 目录下的文件的完整路径
	versionsDir := "versions"
	filePath := filepath.Join(versionsDir, filename)
	log.Printf("filename: %s, filePath: %s", filename, filePath) // 添加日志

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("文件不存在: %s", filePath)
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// 读取文件内容
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("读取文件失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"errorCode":-8,"errorMsg":"Failed to read file: %v"}`, err)))
		return
	}

	// 添加边界检查
	if len(fileContent) < 15 { // 假设切片操作的索引是 0:15
		log.Printf("文件内容长度不足，无法进行切片操作")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"errorCode":-10,"errorMsg":"File content too short"}`)))
		return
	}

	// 设置响应头并发送文件内容
	w.Header().Set("Content-Type", "application/xml") // 假设是 XML 文件，根据实际情况修改
	w.Write(fileContent)

	log.Printf("成功返回 %s 给 %s", filePath, r.RemoteAddr)
}
