package controller

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

const (
	eidHeader = "eid"
)

var (
	sdkPathPattern = regexp.MustCompile(`^/(provider|consumer)/\w+$`)
)

type httpHandler struct {
	endpointID atomic.Int32
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if sdkPathPattern.MatchString(path) {
		h.handleServerRegister(w, r)
		return
	} else if path == providerRegisterPath {
		h.handleProviderRegister(w, r)
		return
	} else if path == serverRegisterPath {
		h.handleReverseProxy(w, r)
		return
	}
	http.Error(w, wrapHTTPError(fmt.Errorf("no such path: %v", path)), http.StatusBadRequest)
}

// 注册server
func (h *httpHandler) handleServerRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	data := &IpPort{}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, wrapHTTPError(err), http.StatusInternalServerError)
		return
	}

	mlog.Debug("new register request", zap.Any("ip", data.IP), zap.Any("port", data.Port))

	httpGateway.mu.Lock()
	key := data.IP + ":" + data.Port
	if lo.Contains(httpGateway.serverAddrs, key) {
		mlog.Infof("addr already registered: %v", key)
		return
	}
	httpGateway.serverAddrs = append(httpGateway.serverAddrs, key)
	httpGateway.mu.Unlock()

	go httpGateway.watchServer(key)
}

func (h *httpHandler) handleProviderRegister(w http.ResponseWriter, r *http.Request) {
	eid := h.endpointID.Add(1)
	r.Header.Add(eidHeader, fmt.Sprintf("%d", eid))
	h.handleReverseProxy(w, r)
}

// 反向代理，将sdk的请求转发给server
func (h *httpHandler) handleReverseProxy(w http.ResponseWriter, r *http.Request) {
	target := httpGateway.lb.GetNextServer()
	reverseTarget, err := url.Parse(target)
	if err != nil {
		http.Error(w, wrapHTTPError(err), http.StatusInternalServerError)
		return
	}
	reverseProxy(reverseTarget).ServeHTTP(w, r)
}

func httpMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().In(Location)
		mlog.Info("received new http request",
			zap.Any("soure addr", r.RemoteAddr), zap.Any("path", r.URL.Path), zap.Any("header", r.Header))

		next.ServeHTTP(w, r)

		mlog.Info("request completed",
			zap.Any("path", r.URL.Path), zap.Any("path", r.URL.Path), zap.Any("time cost", time.Since(start)))
	})
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

func wrapHTTPError(err error) string {
	mlog.Warnf("http request failed: %v", err)
	b, _ := json.Marshal(map[string]string{
		"error": err.Error(),
	})
	return string(b)
}

func reverseProxy(target *url.URL) *httputil.ReverseProxy {
	rr := httputil.NewSingleHostReverseProxy(target)
	originalDirector := rr.Director
	rr.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}
	return rr
}
