package nacos

import (
	"fmt"
	"log"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

var (
	NacosAddr   = "127.0.0.1"
	NacosPort   = 8848
	ServiceName = "gatesvr"
	GroupName   = "DEFAULT_GROUP"
	client      naming_client.INamingClient
)

// 初始化Nacos客户端并注册服务
func Register(instanceID, ip string, port uint64) {
	serverConfigs := []constant.ServerConfig{{
		IpAddr: NacosAddr,
		Port:  uint64(NacosPort),
	}}
	clientConfig := constant.ClientConfig{
		TimeoutMs:           5000,
		BeatInterval:        5000,
		NotLoadCacheAtStart: true,
		LogDir:              "./nacos/log",
		CacheDir:            "./nacos/cache",
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
		ServiceName: ServiceName,
		GroupName:   GroupName,
		Weight:      10,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    map[string]string{"instanceID": instanceID},
	})
	if err != nil {
		log.Fatalf("Nacos服务注册失败: %v", err)
	}
	log.Printf("已注册到Nacos: %s %s:%d", ServiceName, ip, port)
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