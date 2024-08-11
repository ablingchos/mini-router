package common

import (
	"net"

	"git.woa.com/mfcn/ms-go/pkg/util"
)

func GetIpAddr() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")

	if err != nil {
		return "", util.ErrorfWithPos("failed to dial target addr")
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String(), nil
}
