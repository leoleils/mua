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
	NacosAddr       = "mse-df8d5240-p.nacos-ans.mse.aliyuncs.com"
	NacosPort       = 8848
	ServiceName     = "gatesvr"
	GroupName       = "DEFAULT_GROUP"
	client          naming_client.INamingClient
	localInstanceID string // 新增本服务实例ID变量
)

// 初始化Nacos客户端并注册服务
func Register(instanceID, ip string, port uint64) {
	localInstanceID = instanceID // 注册时赋值
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

// 根据服务名称判断是否在线 默认group为DEFAULT_GROUP
func IsServiceOnline(serviceName string) bool {
	res, err := client.SelectAllInstances(vo.SelectAllInstancesParam{
		ServiceName: serviceName,
		GroupName:   GroupName,
	})
	if err != nil {
		log.Printf("Nacos获取实例失败: %v", err)
		return false
	}
	return len(res) > 0
}

// 根据服务名称获取所有服务地址 默认group为DEFAULT_GROUP
func GetServiceAddrs(serviceName string) []string {
	res, err := client.SelectAllInstances(vo.SelectAllInstancesParam{
		ServiceName: serviceName,
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

// 根据实例ID获取服务地址（如未找到返回空字符串）
func GetAddrByInstanceID(instanceID string) string {
	res, err := client.SelectAllInstances(vo.SelectAllInstancesParam{
		ServiceName: ServiceName,
		GroupName:   GroupName,
	})
	if err != nil {
		log.Printf("Nacos获取实例失败: %v", err)
		return ""
	}
	for _, inst := range res {
		if id, ok := inst.Metadata["instanceID"]; ok && id == instanceID {
			return fmt.Sprintf("%s:%d", inst.Ip, inst.Port)
		}
	}
	return ""
}

// 根据实例ID判断实例是否在线
func IsInstanceOnline(instanceID string) bool {
	res, err := client.SelectAllInstances(vo.SelectAllInstancesParam{
		ServiceName: ServiceName,
		GroupName:   GroupName,
	})
	if err != nil {
		log.Printf("Nacos获取实例失败: %v", err)
		return false
	}
	for _, inst := range res {
		if id, ok := inst.Metadata["instanceID"]; ok && id == instanceID {
			return true
		}
	}
	return false
}

// 获取本服务实例ID
func GetLocalInstanceID() string {
	return localInstanceID
}
