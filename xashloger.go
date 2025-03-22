// v25.3.22
package main

import (
	"encoding/hex"
	"fmt"
	"net"
)

var colorReset = "\033[0m"

var colorRed = "\033[31m"
var colorYellow = "\033[33m"
var colorBlue = "\033[34m"

var colorRedBackground = "\033[41m"
var colorWhite = "\033[97m"

func sendToSniffer(Request []byte) {
	conn, err := net.Dial("udp", "127.0.0.1:9777")
	if err != nil {
		println("UDP Error on sending to sniffer")
		return
	}
	// send to server
	fmt.Fprintf(conn, string(Request))
}

func main() {

	addr := net.UDPAddr{
		Port: 1,
		IP:   net.ParseIP("0.0.0.0"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	fmt.Println("Bind on", addr.String())

	buf := make([]byte, 256)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		//buf := buf[16:n]
		sendToSniffer(buf)

		println(colorRedBackground + colorWhite + addr.String() + colorReset + string(buf[8:n]))

		println(colorYellow + hex.Dump((buf[:n])) + colorReset)

		println("")

	}

}
