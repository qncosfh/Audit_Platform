package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// ProgressMessage 进度消息
type ProgressMessage struct {
	TaskID   string `json:"task_id"`
	Progress int    `json:"progress"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Result   string `json:"result,omitempty"`
	Log      string `json:"log,omitempty"`   // 审计日志
	AILog    string `json:"aiLog,omitempty"` // AI交互日志
}

// Upgrader WebSocket升级器
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ProgressManager 进度管理器
type ProgressManager struct {
	clients map[*websocket.Conn]bool
	mu      sync.RWMutex
}

// NewProgressManager 创建新的进度管理器
func NewProgressManager() *ProgressManager {
	return &ProgressManager{
		clients: make(map[*websocket.Conn]bool),
	}
}

// AddClient 添加客户端
func (pm *ProgressManager) AddClient(conn *websocket.Conn) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.clients[conn] = true
	fmt.Printf("客户端已连接: %s\n", conn.RemoteAddr())
}

// RemoveClient 移除客户端
func (pm *ProgressManager) RemoveClient(conn *websocket.Conn) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.clients, conn)
	fmt.Printf("客户端已断开: %s\n", conn.RemoteAddr())
}

// BroadcastProgress 广播进度
func (pm *ProgressManager) BroadcastProgress(taskID string, progress int, status, message, aiLog string, result string) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	msg := ProgressMessage{
		TaskID:   taskID,
		Progress: progress,
		Status:   status,
		Message:  message,
		Result:   result,
		AILog:    aiLog,
	}

	messageData, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("序列化进度消息失败: %v\n", err)
		return
	}

	for conn := range pm.clients {
		if err := conn.WriteMessage(websocket.TextMessage, messageData); err != nil {
			fmt.Printf("发送消息失败: %v\n", err)
			conn.Close()
			delete(pm.clients, conn)
		}
	}
}

// SendProgressToClient 向特定客户端发送进度
func (pm *ProgressManager) SendProgressToClient(conn *websocket.Conn, taskID string, progress int, status, message string, result string) {
	msg := ProgressMessage{
		TaskID:   taskID,
		Progress: progress,
		Status:   status,
		Message:  message,
		Result:   result,
	}

	messageData, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("序列化进度消息失败: %v\n", err)
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, messageData); err != nil {
		fmt.Printf("发送消息失败: %v\n", err)
		conn.Close()
		pm.RemoveClient(conn)
	}
}

// HandleProgressWebSocket 处理WebSocket连接
func (pm *ProgressManager) HandleProgressWebSocket(conn *websocket.Conn) {
	pm.AddClient(conn)
	defer pm.RemoveClient(conn)

	// 保持连接
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// UpdateTaskProgress 更新任务进度（供其他包调用）
func (pm *ProgressManager) UpdateTaskProgress(taskID string, progress int, status, message, aiLog string) {
	pm.BroadcastProgress(taskID, progress, status, message, aiLog, "")
}

// UpdateTaskResult 更新任务结果
func (pm *ProgressManager) UpdateTaskResult(taskID string, result string) {
	pm.BroadcastProgress(taskID, 100, "completed", "审计完成", "", result)
}
