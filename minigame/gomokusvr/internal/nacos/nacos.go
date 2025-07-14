package nacos

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"mua/minigame/gomokusvr/internal/config"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

var namingClient naming_client.INamingClient

// InitNacos 初始化Nacos客户端
func InitNacos() error {
	nacosConfig := config.GetNacosConfig()

	// 创建服务发现客户端配置
	clientConfig := constant.ClientConfig{
		NamespaceId:         "public", // 使用默认命名空间
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "./nacos/log",
		CacheDir:            "./nacos/cache",
		LogLevel:            "info", // 使用默认日志级别
	}

	// 创建服务端配置
	serverConfigs := []constant.ServerConfig{
		{
			IpAddr: nacosConfig.Addr,
			Port:   uint64(nacosConfig.Port),
		},
	}

	// 创建服务发现客户端
	client, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return fmt.Errorf("创建Nacos客户端失败: %v", err)
	}

	namingClient = client
	log.Println("Nacos客户端初始化成功")
	return nil
}

// RegisterService 注册服务到Nacos
func RegisterService() error {
	if namingClient == nil {
		return fmt.Errorf("Nacos客户端未初始化")
	}

	serviceConfig := config.GetServiceConfig()
	nacosConfig := config.GetNacosConfig()

	// 获取本地IP
	ip, err := getLocalIP()
	if err != nil {
		return fmt.Errorf("获取本地IP失败: %v", err)
	}

	// 注册服务实例
	success, err := namingClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        uint64(serviceConfig.Port),
		ServiceName: serviceConfig.Name,
		Weight:      10,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    map[string]string{"version": serviceConfig.Version},
		ClusterName: nacosConfig.Cluster,
		GroupName:   nacosConfig.Group,
	})

	if err != nil {
		return fmt.Errorf("注册服务实例失败: %v", err)
	}

	if !success {
		return fmt.Errorf("注册服务实例失败: 返回false")
	}

	log.Printf("服务注册成功 - 服务名: %s, IP: %s, 端口: %d",
		serviceConfig.Name, ip, serviceConfig.Port)
	return nil
}

// DeregisterService 注销服务
func DeregisterService() error {
	if namingClient == nil {
		return fmt.Errorf("Nacos客户端未初始化")
	}

	serviceConfig := config.GetServiceConfig()
	nacosConfig := config.GetNacosConfig()

	ip, err := getLocalIP()
	if err != nil {
		return fmt.Errorf("获取本地IP失败: %v", err)
	}

	success, err := namingClient.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          ip,
		Port:        uint64(serviceConfig.Port),
		ServiceName: serviceConfig.Name,
		Ephemeral:   true,
		Cluster:     nacosConfig.Cluster,
		GroupName:   nacosConfig.Group,
	})

	if err != nil {
		return fmt.Errorf("注销服务实例失败: %v", err)
	}

	if !success {
		return fmt.Errorf("注销服务实例失败: 返回false")
	}

	log.Printf("服务注销成功 - 服务名: %s", serviceConfig.Name)
	return nil
}

// getLocalIP 获取本地IP地址
func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("未找到有效的本地IP地址")
}

// GetServiceInstances 获取服务实例列表
func GetServiceInstances(serviceName string) ([]model.Instance, error) {
	if namingClient == nil {
		return nil, fmt.Errorf("Nacos客户端未初始化")
	}

	nacosConfig := config.GetNacosConfig()

	instances, err := namingClient.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		GroupName:   nacosConfig.Group,
		HealthyOnly: true,
	})

	if err != nil {
		return nil, fmt.Errorf("获取服务实例失败: %v", err)
	}

	return instances, nil
}

// GetServiceAddress 根据服务名获取一个可用的服务地址
func GetServiceAddress(serviceName string) (string, error) {
	instances, err := GetServiceInstances(serviceName)
	if err != nil {
		return "", err
	}

	if len(instances) == 0 {
		return "", fmt.Errorf("没有可用的服务实例: %s", serviceName)
	}

	// 简单返回第一个实例
	instance := instances[0]
	return net.JoinHostPort(instance.Ip, strconv.FormatUint(instance.Port, 10)), nil
}

// GetClient 获取Nacos客户端
func GetClient() naming_client.INamingClient {
	return namingClient
}
