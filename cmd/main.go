package main

import (
	"fmt"
	"os"
)

func main() {
	// conn, err := net.Dial("udp", "8.8.8.8:80")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// defer conn.Close()

	// localAddr := conn.LocalAddr().(*net.UDPAddr)

	// fmt.Println(localAddr.IP.String())
	host, _ := os.Hostname()
	fmt.Print(host)
}
