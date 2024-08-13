package provider

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/common"
	"git.woa.com/kefuai/mini-router/pkg/proto/providerpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	healthCheckAddr = "localhost:5100"
)

var (
	once     sync.Once
	provider *Provider
)

type ProviderConfig providerpb.RegisterRequest

type Provider struct {
	config  *ProviderConfig
	eid     int64
	leaseId int64
	ctx     context.Context
	cancel  context.CancelFunc

	discoverClient providerpb.ProviderServiceClient
}

func (p *Provider) initializeProviderForTest(port string, eid int64) error {
	config := &providerpb.RegisterRequest{
		GroupName: "test",
		HostName:  "test1",
		Port:      port,
		Weight:    10,
		Timeout:   5,
	}
	p.config = (*ProviderConfig)(config)
	// 获取ip
	ip, err := common.GetIpAddr()
	if err != nil {
		return util.ErrorWithPos(err)
	}
	p.config.Ip = ip

	p.ctx, p.cancel = context.WithCancel(context.Background())
	if err := p.grpcConnect(); err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}

func (p *Provider) initializeProvider(configPath string) error {
	// 读配置
	bytes, err := common.LoadYAML(configPath)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	if len(bytes) != 1 {
		return util.ErrorfWithPos("wrong length of config: %v", len(bytes))
	}
	config := &providerpb.RegisterRequest{}
	if err := json.Unmarshal(bytes[0], config); err != nil {
		return util.ErrorWithPos(err)
	}
	mlog.Debug("load config successfully", zap.Any("config", config))
	p.config = (*ProviderConfig)(config)
	// 获取ip
	ip, err := common.GetIpAddr()
	if err != nil {
		return util.ErrorWithPos(err)
	}
	p.config.Ip = ip

	p.ctx, p.cancel = context.WithCancel(context.Background())
	if err := p.grpcConnect(); err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}

func (p *Provider) grpcConnect() error {
	conn, err := grpc.NewClient(healthCheckAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	p.discoverClient = providerpb.NewProviderServiceClient(conn)
	return nil
}

func (p *Provider) register() error {
	resp, err := p.discoverClient.Register(p.ctx, (*providerpb.RegisterRequest)(p.config))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	p.eid = resp.GetEid()
	p.leaseId = resp.GetLeaseId()
	// go p.serverGrpc()
	go p.controlLoop()

	return nil
}

// func (p *Provider) serverGrpc() {
// 	lis, err := net.Listen("tcp", ":" + p.config.Port)
// 	if err != nil {
// 		mlog.Fatalf("failed to listen port %v: %v", p.config.Port, err)
// 	}
// 	s := grpc.NewServer()

// }

func (p *Provider) controlLoop() {
	ticker := time.NewTicker(time.Duration(p.config.Timeout>>1) * time.Second)
	for {
		select {
		case <-ticker.C:
			err := p.heartbeat()
			if err != nil {
				mlog.Debugf("failed to send heartbeat, start to retry: %v", err)
				// 重试3次
				fail := true
				for i := 1; i <= 3; i++ {
					// 每隔0.5s重试一次
					time.Sleep(time.Second / 2)
					if err := p.heartbeat(); err == nil {
						fail = false
						break
					}
					if fail {
						mlog.Fatal("failed to send heartbeat with retry")
					}
				}
			}
			mlog.Debug("sent heartbeat")
		case <-p.ctx.Done():
			mlog.Info("control loop stopped")
			go p.unregister()
			return
		}
	}
}

func (p *Provider) heartbeat() error {
	if _, err := p.discoverClient.Heartbeat(p.ctx, &providerpb.HeartbeatRequest{
		GroupName: p.config.GroupName,
		HostName:  p.config.HostName,
		Eid:       p.eid,
		LeaseId:   p.leaseId,
	}); err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}

func (p *Provider) unregister() error {
	if _, err := p.discoverClient.Unregister(p.ctx, &providerpb.UnregisterRequest{
		GroupName: p.config.GroupName,
		HostName:  p.config.HostName,
		Eid:       p.eid,
		LeaseId:   p.leaseId,
	}); err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}

// const (
// 	RegisterPath   = "/provider/register"
// 	HeartbeatPath  = "/provider/heartbeat"
// 	UnregisterPath = "/provider/unregister"
// )

// func (p *Provider) unregister() {
// 	identity := &providerpb.HeartbeatRequest{
// 		GroupName: p.Group,
// 		HostName:  p.Host,
// 		Eid:       p.eid.Eid,
// 	}
// 	reqbody, err := json.Marshal(identity)
// 	if err != nil {
// 		mlog.Warnf("failed to marshal to yaml, err: %v", err)
// 		return
// 	}
// 	_, err = p.sendPostRequest(UnregisterPath, reqbody)
// 	if err != nil {
// 		mlog.Warnf("failed to unregister, err: %v", err)
// 		return
// 	}
// 	mlog.Info("unregister successfully")
// }

// func (p *Provider) heartbeat() error {
// 	identity := &providerpb.HeartbeatRequest{
// 		GroupName: p.Group,
// 		HostName:  p.Host,
// 		Eid:       p.eid.Eid,
// 	}
// 	reqBody, err := json.Marshal(identity)
// 	if err != nil {
// 		return util.ErrorWithPos(err)
// 	}
// 	_, err = p.sendPostRequest(HeartbeatPath, reqBody)
// 	if err != nil {
// 		return util.ErrorWithPos(err)
// 	}

// 	return nil
// }

// func (p *Provider) register(configPath string) error {
// ip, err := common.GetIpAddr()
// if err != nil {
// 	return util.ErrorWithPos(err)
// }
// p.Ip = ip
// // 读配置
// configBytes, err := os.ReadFile(configPath)
// if err != nil {
// 	return util.ErrorWithPos(err)
// }
// // yaml转json
// identity := Eid{}
// err = yaml.Unmarshal(configBytes, &identity)
// if err != nil {
// 	return util.ErrorWithPos(err)
// }
// jsonBytes, err := json.Marshal(identity)
// if err != nil {
// 	return util.ErrorWithPos(err)
// }
// 	// 向controller发送注册请求
// 	respBody, err := p.sendPostRequest(RegisterPath, jsonBytes)
// 	if err != nil {
// 		return util.ErrorWithPos(err)
// 	}
// 	// 解析请求回包
// 	if err := json.Unmarshal(respBody, &identity); err != nil {
// 		return util.ErrorWithPos(err)
// 	}
// 	p.eid = &identity
// 	// 开始心跳上报
// 	go p.controlLoop()
// 	return nil
// }

// func (p *Provider) sendPostRequest(path string, reqBody []byte) ([]byte, error) {
// 	req, err := http.NewRequest("POST", p.ControllerDomain+path, bytes.NewBuffer(reqBody))
// 	if err != nil {
// 		return nil, util.ErrorWithPos(err)
// 	}
// 	req.Header.Set("Content-Type", "application/json")
// 	resp, err := p.httpClient.Do(req)
// 	if err != nil {
// 		return nil, util.ErrorWithPos(err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != 200 {
// 		return nil, util.ErrorfWithPos("failed to send register request, status: %v, body: %v", resp.Status, resp.Body)
// 	}
// 	bodyBytes, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, util.ErrorWithPos(err)
// 	}
// 	return bodyBytes, nil
// }
