package nacos

import (
	"fmt"
	"log"

	"mua/gatesvr/config"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

var (
	NacosAddr   = "mse-df8d5240-p.nacos-ans.mse.aliyuncs.com"
	NacosPort   = 8848
	ServiceName = "gatesvr"
	GroupName   = "DEFAULT_GROUP"
	client      naming_client.INamingClient
)

// 初始化Nacos客户端并注册服务
func Register(instanceID, ip string, port uint64) {
	cfg := config.GetConfig().Nacos
	serverConfigs := []constant.ServerConfig{{
		IpAddr: cfg.Addr,
		Port:   cfg.Port,
	}}
	clientConfig := constant.ClientConfig{
		TimeoutMs:           5000,
		BeatInterval:        5000,
		NotLoadCacheAtStart: true,
		LogDir:              "./nacos/log",
		CacheDir:            "./nacos/cache",
		Username:            cfg.Username,
		Password:            cfg.Password,
	}
	var err error
	client, err = clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		log.Fatalf("Nacos客户端初始化失败: %v", err)
	}
	_, err = client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        port,
		ServiceName: cfg.Service,
		GroupName:   cfg.Group,
		Weight:      10,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    map[string]string{"instanceID": instanceID},
	})
	if err != nil {
		log.Fatalf("Nacos服务注册失败: %v", err)
	}
	log.Printf("已注册到Nacos: %s %s:%d", cfg.Service, ip, port)
}

// 获取所有gatesvr实例地址
func GetAllInstances() []string {
	res, err := client.SelectAllInstances(vo.SelectAllInstancesParam{
		ServiceName: ServiceName,
		GroupName:   GroupName,
	})
	if err != nil {
		log.Printf("Nacos获取实例失败: %v", err)
		return nil
	}
	var addrs []string
	for _, inst := range res {
		addrs = append(addrs, fmt.Sprintf("%s:%d", inst.Ip, inst.Port))
	}
	return addrs
}

// 获取所有gatesvr实例ID（从metadata.instanceID获取）
func GetAllInstanceIDs() []string {
	res, err := client.SelectAllInstances(vo.SelectAllInstancesParam{
		ServiceName: ServiceName,
		GroupName:   GroupName,
	})
	if err != nil {
		log.Printf("Nacos获取实例失败: %v", err)
		return nil
	}
	var ids []string
	for _, inst := range res {
		if id, ok := inst.Metadata["instanceID"]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// 监听 gatesvr 服务变更
func SubscribeServiceChange(callback func(onlineAddrs []string)) {
	go func() {
		_ = client.Subscribe(&vo.SubscribeParam{
			ServiceName: ServiceName,
			GroupName:   GroupName,
			SubscribeCallback: func(services []model.Instance, err error) {
				if err != nil {
					log.Printf("Nacos订阅回调错误: %v", err)
					return
				}
				var addrs []string
				for _, inst := range services {
					addrs = append(addrs, fmt.Sprintf("%s:%d", inst.Ip, inst.Port))
				}
				callback(addrs)
			},
		})
	}()
}
