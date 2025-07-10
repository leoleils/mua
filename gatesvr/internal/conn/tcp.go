package conn

import (
	"log"
	"mua/gatesvr/config"
	"net"
	"time"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

const (
	HeartbeatInterval = 30 * time.Second
	HeartbeatTimeout  = 60 * time.Second
)

// StartTCPServer 启动TCP监听
func StartTCPServer(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("TCP监听失败: %v", err)
	}
	log.Printf("TCP服务已启动，监听: %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("接受连接失败: %v", err)
			continue
		}
		go func(c net.Conn) {
			adapter := NewTCPConnAdapter(c)
			HandleConnection(adapter, ProtocolTCP, config.GetConfig().EnableTokenCheck)
		}(conn)
	}
}
