package rpc

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"

	"mua/gatesvr/internal/pb"
)

func PushToClientRemote(addr string, req *pb.PushRequest) error {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		log.Printf("gRPC连接失败[%s]: %v", addr, err)
		return err
	}
	defer conn.Close()
	client := pb.NewGateSvrClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = client.PushToClient(ctx, req)
	if err != nil {
		log.Printf("远程PushToClient失败[%s]: %v", addr, err)
	}
	return err
}

func KickPlayerRemote(addr, playerID, reason string) error {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		log.Printf("gRPC连接失败[%s]: %v", addr, err)
		return err
	}
	defer conn.Close()
	client := pb.NewGateSvrClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = client.KickPlayer(ctx, &pb.KickPlayerRequest{
		PlayerId: playerID,
		Reason:  reason,
	})
	if err != nil {
		log.Printf("远程KickPlayer失败[%s]: %v", addr, err)
	}
	return err
}

func ForwardMessageRemote(addr, playerID string, payload []byte) error {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		log.Printf("gRPC连接失败[%s]: %v", addr, err)
		return err
	}
	defer conn.Close()
	client := pb.NewGateSvrClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = client.ForwardMessage(ctx, &pb.ForwardMessageRequest{
		PlayerId: playerID,
		Payload:  payload,
	})
	if err != nil {
		log.Printf("远程ForwardMessage失败[%s]: %v", addr, err)
	}
	return err
}

// TODO: 检查pb.GateSvrClient是否有PushToClient方法，如无需重新生成pb文件 