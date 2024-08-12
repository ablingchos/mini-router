package provider

// 三级结构：group（组）->host（服务）->endpoint（节点）
type Group struct {
	Name  string           `yaml:"name"`  // 路由组名称（类似namespace的概念）
	Gid   uint32           `yaml:"gid"`   // 路由组id
	Hosts map[string]*Host `yaml:"hosts"` // host表
}

type Host struct {
	Name         string               `yaml:"name"`         // host名
	RoutingRules RoutingRule          `yaml:"routingRules"` // 基础路由规则
	MatchRules   []*MatchRule         `yaml:"matchRules"`   // 键值路由规则
	Weight       int32                `yaml:"weight"`       // 权重
	Endpoints    map[uint32]*Endpoint `yaml:"endpoints"`    // 节点表
	HealthyCount uint32               `yaml:"healthyCount"` // 存活节点数
	Timeout      uint8                `yaml:"timeout"`      // 心跳超时时间
}

type Endpoint struct {
	Eid       uint32 `yaml:"eid"`       // 节点id
	Address   string `yaml:"address"`   // ip
	Port      string `yaml:"port"`      // port
	IsHealthy bool   `yaml:"isHealthy"` // 是否存活
	LastCheck int64  `yaml:"lastCheck"` // 上一次心跳时间
}

type MatchRule struct {
	Method   MatchMethod `yaml:"method"`   // 匹配模式
	Template string      `yaml:"template"` // 匹配模板
}

type RoutingRule uint32

const (
	ConsistentHashing RoutingRule = iota // 一致性哈希
	Random                               // 随机
	Weight                               // 权重
	Destination                          // 指定目标
)

type MatchMethod uint32

const (
	Prefix MatchMethod = iota // 前缀匹配
	Exact                     // 完全匹配
)
