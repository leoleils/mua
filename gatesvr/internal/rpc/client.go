package rpc

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"mua/gatesvr/internal/pb"
)

// PushToClientRemote 远程推送消息到客户端
func PushToClientRemote(addr string, req *pb.PushRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		log.Printf("gRPC连接失败[%s]: %v", addr, err)
		return err
	}
	defer conn.Close()

	client := pb.NewGateSvrClient(conn)
	_, err = client.PushToClient(ctx, req)
	if err != nil {
		log.Printf("远程PushToClient失败[%s]: %v", addr, err)
	}
	return err
}

// KickPlayerRemote 远程踢玩家下线
func KickPlayerRemote(addr, playerID, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		log.Printf("gRPC连接失败[%s]: %v", addr, err)
		return err
	}
	defer conn.Close()

	client := pb.NewGateSvrClient(conn)
	_, err = client.KickPlayer(ctx, &pb.KickPlayerRequest{
		PlayerId: playerID,
		Reason:   reason,
	})
	if err != nil {
		log.Printf("远程KickPlayer失败[%s]: %v", addr, err)
	}
	return err
}

// ForwardMessageRemote 远程转发消息
func ForwardMessageRemote(addr, playerID string, payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		log.Printf("gRPC连接失败[%s]: %v", addr, err)
		return err
	}
	defer conn.Close()

	client := pb.NewGateSvrClient(conn)
	_, err = client.ForwardMessage(ctx, &pb.ForwardMessageRequest{
		PlayerId: playerID,
		Payload:  payload,
	})
	if err != nil {
		log.Printf("远程ForwardMessage失败[%s]: %v", addr, err)
	}
	return err
}
