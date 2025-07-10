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

// StartWSServer 启动WebSocket服务
func StartWSServer(addr string) {
	http.HandleFunc("/ws", wsUpgradeHandler)
	log.Printf("WebSocket服务已启动，监听: %s/ws", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("WebSocket监听失败: %v", err)
	}
}

// wsUpgradeHandler 处理WebSocket协议升级
func wsUpgradeHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}
	adapter := NewWSConnAdapter(conn, r)
	go HandleConnection(
		adapter,
		ProtocolWS,
		config.GetConfig().EnableTokenCheck,
	)
}
