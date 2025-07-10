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
	localIP         string // 新增本服务实例IP变量
)

// 初始化Nacos客户端并注册服务到注册中心
func Register(instanceID, ip string, port uint64) {
	localInstanceID = instanceID // 注册时赋值
	localIP = ip                 // 注册时赋值
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

// 监听 gatesvr 服务变更并通过回调函数通知
func SubscribeGatesvrChange(callback func(onlineAddrs []string)) {
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

// 根据服务名称判断是否在线（默认group为DEFAULT_GROUP）
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

// 根据服务名称获取所有服务地址（默认group为DEFAULT_GROUP）
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

// 根据实例ID在gatesvr服务中获取地址（如未找到返回空字符串）
func GetGatesvrAddrByInstanceID(instanceID string) string {
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

// 根据实例ID判断gatesvr实例是否在线
func IsGatesvrInstanceOnline(instanceID string) bool {
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

// 获取本服务实例IP
func GetLocalIP() string {
	return localIP
}

// 负载均衡策略
type LoadBalanceStrategy int

const (
	// 轮询策略
	RoundRobin LoadBalanceStrategy = iota
	// 权重策略
	Weighted
)

// 服务实例信息
type ServiceInstance struct {
	IP     string
	Port   uint64
	Weight float64
	Addr   string
}

// 负载均衡器
type LoadBalancer struct {
	serviceName string
	groupName   string // 新增分组名称
	strategy    LoadBalanceStrategy
	// 轮询计数器
	roundRobinIndex int
	// 权重相关
	weightedInstances []ServiceInstance
	currentWeights    []float64
}

// 创建负载均衡器
func NewLoadBalancer(serviceName string, strategy LoadBalanceStrategy) *LoadBalancer {
	return &LoadBalancer{
		serviceName:     serviceName,
		groupName:       GroupName, // 使用默认分组
		strategy:        strategy,
		roundRobinIndex: 0,
	}
}

// 创建负载均衡器（支持分组）
func NewLoadBalancerWithGroup(serviceName, groupName string, strategy LoadBalanceStrategy) *LoadBalancer {
	// 分组为空时使用默认分组
	if groupName == "" {
		groupName = GroupName
	}
	return &LoadBalancer{
		serviceName:     serviceName,
		groupName:       groupName,
		strategy:        strategy,
		roundRobinIndex: 0,
	}
}

// 获取服务实例列表（带权重信息）
func (lb *LoadBalancer) getServiceInstances() ([]ServiceInstance, error) {
	res, err := client.SelectAllInstances(vo.SelectAllInstancesParam{
		ServiceName: lb.serviceName,
		GroupName:   lb.groupName, // 使用负载均衡器的分组
	})
	if err != nil {
		return nil, fmt.Errorf("获取服务实例失败: %v", err)
	}

	if len(res) == 0 {
		return nil, fmt.Errorf("服务 %s@%s 没有可用实例", lb.serviceName, lb.groupName)
	}

	instances := make([]ServiceInstance, 0, len(res))
	for _, inst := range res {
		// 只添加健康且启用的实例
		if inst.Healthy && inst.Enable {
			instances = append(instances, ServiceInstance{
				IP:     inst.Ip,
				Port:   inst.Port,
				Weight: inst.Weight,
				Addr:   fmt.Sprintf("%s:%d", inst.Ip, inst.Port),
			})
		}
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("服务 %s@%s 没有健康的实例", lb.serviceName, lb.groupName)
	}

	return instances, nil
}

// 轮询算法选择实例
func (lb *LoadBalancer) roundRobinSelect(instances []ServiceInstance) string {
	if len(instances) == 0 {
		return ""
	}

	// 使用取模实现轮询
	index := lb.roundRobinIndex % len(instances)
	lb.roundRobinIndex++

	// 防止计数器溢出
	if lb.roundRobinIndex >= len(instances)*1000 {
		lb.roundRobinIndex = 0
	}

	return instances[index].Addr
}

// 加权轮询算法选择实例 (Weighted Round Robin)
func (lb *LoadBalancer) weightedSelect(instances []ServiceInstance) string {
	if len(instances) == 0 {
		return ""
	}

	// 如果只有一个实例，直接返回
	if len(instances) == 1 {
		return instances[0].Addr
	}

	// 初始化或更新权重实例列表
	if len(lb.weightedInstances) != len(instances) || lb.needUpdateWeights(instances) {
		lb.weightedInstances = instances
		lb.currentWeights = make([]float64, len(instances))
		for i := range instances {
			lb.currentWeights[i] = 0
		}
	}

	// 加权轮询算法实现
	// 1. 每个实例的当前权重 += 配置权重
	totalWeight := 0.0
	for i, inst := range lb.weightedInstances {
		lb.currentWeights[i] += inst.Weight
		totalWeight += inst.Weight
	}

	// 2. 找到当前权重最高的实例
	maxWeightIndex := 0
	for i := 1; i < len(lb.currentWeights); i++ {
		if lb.currentWeights[i] > lb.currentWeights[maxWeightIndex] {
			maxWeightIndex = i
		}
	}

	// 3. 选中实例的当前权重 -= 总权重
	lb.currentWeights[maxWeightIndex] -= totalWeight

	return lb.weightedInstances[maxWeightIndex].Addr
}

// 检查是否需要更新权重信息
func (lb *LoadBalancer) needUpdateWeights(instances []ServiceInstance) bool {
	if len(lb.weightedInstances) != len(instances) {
		return true
	}

	for i, inst := range instances {
		if i >= len(lb.weightedInstances) ||
			lb.weightedInstances[i].Addr != inst.Addr ||
			lb.weightedInstances[i].Weight != inst.Weight {
			return true
		}
	}
	return false
}

// 获取一个 RPC 地址（根据负载均衡策略）
func (lb *LoadBalancer) GetRPCAddr() (string, error) {
	instances, err := lb.getServiceInstances()
	if err != nil {
		return "", err
	}

	switch lb.strategy {
	case RoundRobin:
		addr := lb.roundRobinSelect(instances)
		if addr != "" {
			log.Printf("[负载均衡-轮询] 选择服务 %s@%s 实例: %s", lb.serviceName, lb.groupName, addr)
		}
		return addr, nil
	case Weighted:
		addr := lb.weightedSelect(instances)
		if addr != "" {
			log.Printf("[负载均衡-权重] 选择服务 %s@%s 实例: %s", lb.serviceName, lb.groupName, addr)
		}
		return addr, nil
	default:
		return "", fmt.Errorf("不支持的负载均衡策略: %v", lb.strategy)
	}
}

// ===== 便捷函数 =====

// 全局负载均衡器管理
var (
	loadBalancers = make(map[string]*LoadBalancer)
)

// 通过服务名获取 RPC 地址（轮询）
func GetRPCAddrRoundRobin(serviceName string) (string, error) {
	key := serviceName + "_roundrobin"
	lb, exists := loadBalancers[key]
	if !exists {
		lb = NewLoadBalancer(serviceName, RoundRobin)
		loadBalancers[key] = lb
	}
	return lb.GetRPCAddr()
}

// 通过服务名获取 RPC 地址（权重）
func GetRPCAddrWeighted(serviceName string) (string, error) {
	key := serviceName + "_weighted"
	lb, exists := loadBalancers[key]
	if !exists {
		lb = NewLoadBalancer(serviceName, Weighted)
		loadBalancers[key] = lb
	}
	return lb.GetRPCAddr()
}

// 通用接口：根据策略获取 RPC 地址
func GetRPCAddr(serviceName string, strategy LoadBalanceStrategy) (string, error) {
	switch strategy {
	case RoundRobin:
		return GetRPCAddrRoundRobin(serviceName)
	case Weighted:
		return GetRPCAddrWeighted(serviceName)
	default:
		return "", fmt.Errorf("不支持的负载均衡策略: %v", strategy)
	}
}

// 清理负载均衡器缓存（在服务实例变更时调用）
func ClearLoadBalancerCache(serviceName string) {
	delete(loadBalancers, serviceName+"_roundrobin")
	delete(loadBalancers, serviceName+"_weighted")
	log.Printf("已清理服务 %s 的负载均衡器缓存", serviceName)
}

// ===== 支持分组的便捷函数 =====

// 通过服务名+分组获取 RPC 地址（轮询）
func GetRPCAddrRoundRobinWithGroup(serviceName, groupName string) (string, error) {
	if groupName == "" {
		groupName = GroupName
	}
	key := serviceName + "@" + groupName + "_roundrobin"
	lb, exists := loadBalancers[key]
	if !exists {
		lb = NewLoadBalancerWithGroup(serviceName, groupName, RoundRobin)
		loadBalancers[key] = lb
	}
	return lb.GetRPCAddr()
}

// 通过服务名+分组获取 RPC 地址（权重）
func GetRPCAddrWeightedWithGroup(serviceName, groupName string) (string, error) {
	if groupName == "" {
		groupName = GroupName
	}
	key := serviceName + "@" + groupName + "_weighted"
	lb, exists := loadBalancers[key]
	if !exists {
		lb = NewLoadBalancerWithGroup(serviceName, groupName, Weighted)
		loadBalancers[key] = lb
	}
	return lb.GetRPCAddr()
}

// 通用接口：根据策略获取 RPC 地址（支持分组）
func GetRPCAddrWithGroup(serviceName, groupName string, strategy LoadBalanceStrategy) (string, error) {
	switch strategy {
	case RoundRobin:
		return GetRPCAddrRoundRobinWithGroup(serviceName, groupName)
	case Weighted:
		return GetRPCAddrWeightedWithGroup(serviceName, groupName)
	default:
		return "", fmt.Errorf("不支持的负载均衡策略: %v", strategy)
	}
}

// 清理负载均衡器缓存（支持分组）
func ClearLoadBalancerCacheWithGroup(serviceName, groupName string) {
	if groupName == "" {
		groupName = GroupName
	}
	key := serviceName + "@" + groupName
	delete(loadBalancers, key+"_roundrobin")
	delete(loadBalancers, key+"_weighted")
	log.Printf("已清理服务 %s@%s 的负载均衡器缓存", serviceName, groupName)
}

// ===== 通过实例ID获取地址的接口 =====

// 通过实例ID获取 RPC 地址（在指定服务和分组中查找）
func GetRPCAddrByInstanceID(serviceName, groupName, instanceID string) (string, error) {
	if groupName == "" {
		groupName = GroupName
	}

	res, err := client.SelectAllInstances(vo.SelectAllInstancesParam{
		ServiceName: serviceName,
		GroupName:   groupName,
	})
	if err != nil {
		return "", fmt.Errorf("获取服务实例失败: %v", err)
	}

	for _, inst := range res {
		// 检查实例ID是否匹配
		if id, ok := inst.Metadata["instanceID"]; ok && id == instanceID {
			// 检查实例是否健康且启用
			if inst.Healthy && inst.Enable {
				addr := fmt.Sprintf("%s:%d", inst.Ip, inst.Port)
				log.Printf("[实例ID查找] 找到实例 %s 在服务 %s@%s: %s", instanceID, serviceName, groupName, addr)
				return addr, nil
			} else {
				return "", fmt.Errorf("实例 %s 在服务 %s@%s 中不健康或已禁用", instanceID, serviceName, groupName)
			}
		}
	}

	return "", fmt.Errorf("在服务 %s@%s 中未找到实例ID: %s", serviceName, groupName, instanceID)
}

// 通过实例ID获取 RPC 地址（仅在默认分组中查找）
func GetRPCAddrByInstanceIDDefault(serviceName, instanceID string) (string, error) {
	return GetRPCAddrByInstanceID(serviceName, GroupName, instanceID)
}

// 通过实例ID获取 RPC 地址（全局查找，会搜索所有已知的服务）
func GetRPCAddrByInstanceIDGlobal(instanceID string) (string, error) {
	// 尝试在 gatesvr 服务中查找（这是当前服务）
	if addr := GetGatesvrAddrByInstanceID(instanceID); addr != "" {
		log.Printf("[全局实例ID查找] 在 gatesvr 服务中找到实例 %s: %s", instanceID, addr)
		return addr, nil
	}

	// 可以在这里添加其他已知服务的查找逻辑
	// 例如：userservice, gameservice 等
	commonServices := []string{"userservice", "gameservice", "payservice", "chatservice"}

	for _, serviceName := range commonServices {
		if addr, err := GetRPCAddrByInstanceIDDefault(serviceName, instanceID); err == nil {
			log.Printf("[全局实例ID查找] 在服务 %s 中找到实例 %s: %s", serviceName, instanceID, addr)
			return addr, nil
		}
	}

	return "", fmt.Errorf("全局范围内未找到实例ID: %s", instanceID)
}

// ===== 扩展的实例信息获取接口 =====

// 获取指定服务和分组的所有实例地址
func GetServiceAddrsWithGroup(serviceName, groupName string) []string {
	if groupName == "" {
		groupName = GroupName
	}

	res, err := client.SelectAllInstances(vo.SelectAllInstancesParam{
		ServiceName: serviceName,
		GroupName:   groupName,
	})
	if err != nil {
		log.Printf("Nacos获取实例失败: %v", err)
		return nil
	}

	var addrs []string
	for _, inst := range res {
		if inst.Healthy && inst.Enable {
			addrs = append(addrs, fmt.Sprintf("%s:%d", inst.Ip, inst.Port))
		}
	}
	return addrs
}

// 获取指定服务和分组的所有实例详细信息
func GetServiceInstancesWithGroup(serviceName, groupName string) ([]ServiceInstance, error) {
	if groupName == "" {
		groupName = GroupName
	}

	res, err := client.SelectAllInstances(vo.SelectAllInstancesParam{
		ServiceName: serviceName,
		GroupName:   groupName,
	})
	if err != nil {
		return nil, fmt.Errorf("获取服务实例失败: %v", err)
	}

	instances := make([]ServiceInstance, 0, len(res))
	for _, inst := range res {
		if inst.Healthy && inst.Enable {
			instances = append(instances, ServiceInstance{
				IP:     inst.Ip,
				Port:   inst.Port,
				Weight: inst.Weight,
				Addr:   fmt.Sprintf("%s:%d", inst.Ip, inst.Port),
			})
		}
	}

	return instances, nil
}
