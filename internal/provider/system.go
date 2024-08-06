package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/providerpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"gopkg.in/yaml.v3"
)

const (
	RegisterPath   = "/provider/register"
	HeartbeatPath  = "/provider/heartbeat"
	UnregisterPath = "/provider/unregister"
)

func (p *Provider) register(configPath string) error {
	ip, err := getIpAddr()
	if err != nil {
		return util.ErrorWithPos(err)
	}
	p.Ip = ip
	// 读配置
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	// yaml转json
	identity := Eid{}
	err = yaml.Unmarshal(configBytes, &identity)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	jsonBytes, err := json.Marshal(identity)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	// 向controller发送注册请求
	respBody, err := p.sendPostRequest(RegisterPath, jsonBytes)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	// 解析请求回包
	if err := json.Unmarshal(respBody, &identity); err != nil {
		return util.ErrorWithPos(err)
	}
	p.eid = &identity
	// 开始心跳上报
	go p.controlLoop()
	return nil
}

func (p *Provider) controlLoop() {
	ticker := time.NewTicker(time.Duration(p.Timeout) * time.Second)
	select {
	case <-ticker.C:
		err := p.heartbeat()
		if err != nil {
			mlog.Debugf("failed to send heartbeat, start to retry: %v", err)
			// 重试3次
			for i := 1; i <= 3; i++ {
				// 每隔1秒重试一次
				time.Sleep(time.Second)
				err = p.heartbeat()
				if err == nil {
					break
				}
			}
		}
	case <-p.stopCh:
		mlog.Info("received stop signal, start to unregister")
		go p.unregister()
		break
	}
}

func (p *Provider) heartbeat() error {
	identity := &providerpb.HeartbeatRequest{
		Eid: ,
	}
	identity.e
		reqBody, err := json.Marshal(p.identity)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	_, err = p.sendPostRequest(HeartbeatPath, reqBody)
	if err != nil {
		return util.ErrorWithPos(err)
	}

	return nil
}

func (p *Provider) unregister() {
	reqbody, err := yaml.Marshal(p.identity)
	if err != nil {
		mlog.Warnf("failed to marshal to yaml, err: %v", err)
		return
	}
	_, err = p.sendPostRequest(UnregisterPath, reqbody)
	if err != nil {
		mlog.Warnf("failed to unregister, err: %v", err)
		return
	}
	mlog.Info("unregister successfully")
}

func getIpAddr() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")

	if err != nil {
		fmt.Println(err)
		return "", util.ErrorfWithPos("failed to dial target addr")
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String(), nil
}

func (p *Provider) sendPostRequest(path string, reqBody []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", p.ControllerDomain+path, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, util.ErrorfWithPos("failed to send register request, status: %v, body: %v", resp.Status, resp.Body)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	return bodyBytes, nil
}
