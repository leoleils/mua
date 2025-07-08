package conn

import (
	"log"
	"net/http"

	"mua/gatesvr/config"

	"github.com/gorilla/websocket"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WebSocket玩家连接结构
// 可与TCP共用PlayerConn结构

// 启动WebSocket服务
func StartWSServer(addr string) {
	http.HandleFunc("/ws", wsUpgradeHandler)
	log.Printf("WebSocket服务已启动，监听: %s/ws", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("WebSocket监听失败: %v", err)
	}
}

// 只负责升级协议和适配器创建
func wsUpgradeHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}
	adapter := NewWSConnAdapter(conn, r)
	go HandleConnection(
		adapter,
		wsHandlerRegistry{},
		config.GetConfig().EnableTokenCheck,
		config.GetConfig().EnableIPWhitelist,
	)
}

// WS消息分发注册表
var handlersWS = make(map[int32]HandlerFuncGeneric)

func RegisterHandlerWS(msgType int32, handler HandlerFuncGeneric) {
	handlersWS[msgType] = handler
}

type wsHandlerRegistry struct{}

func (wsHandlerRegistry) GetHandler(msgType int32) HandlerFuncGeneric {
	return handlersWS[msgType]
}
