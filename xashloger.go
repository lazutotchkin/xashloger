// v25.3.24
package main

import (
	"fmt"
	"net"
	"strings"
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

func T(Str string) string {
	return strings.Trim(Str, " ")
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

		LogS := string(buf[8:n])
		//LogDateTime := string(buf[9:29])
		println(colorRedBackground + colorWhite + addr.String() + colorReset + " " + LogS)
		//println(LogDateTime)

		//LogS = string(buf[31:n])
		//println(LogS)

		if strings.Index(LogS, "connected, address") > 0 {
			println("connected, address")
		}
		if strings.Index(LogS, "entered the game") > 0 {
			println("entered the game")
		}
		if strings.Index(LogS, "disconnected") > 0 {
			println("disconnected")
		}
		if strings.Index(LogS, " killed ") > 0 {
			println("killed")

			// "artemiy<526><ID_46046f7fe77d5341a4ef172079e6aad><526>" killed "jjjjjjjj<527><ID_46046f7fe77d5341a4ef172079e6aad><527>" with "9mmAR"
			Test := strings.Split(LogS, "\"")
			println("WhoNickname: " + strings.Split(T(Test[1]), "<")[0])
			//println("WhoSteamID: " + strings.Trim(strings.Split(T(Test[1]), "<")[2], ">"))
			println("WhomNickname: " + strings.Split(T(Test[3]), "<")[0])
			//println("WhomSteamID: " + strings.Trim(strings.Split(T(Test[3]), "<")[3], ">"))
			println("Weapon: " + T(Test[5]))
		}
		if strings.Index(LogS, "committed suicide with") > 0 {
			println("committed suicide with")
			Test := strings.Split(LogS, "\"")
			println("WhoNickname: " + T(Test[1]))
			println("Weapon: " + T(Test[3]))
		}

		//println(colorYellow + hex.Dump((buf[:n])) + colorReset)

		println("")

	}

}
