package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Waziup/single_chan_pkt_fwd/SX127X"
)

func main() {
	log.SetFlags(0)
	// SX127X.LogLevel = SX127X.LogLevelDebug

	radio, err := SX127X.Discover()
	if err != nil {
		fmt.Println("Error: Can not activate radio:", err)
		os.Exit(1)
	}

	log.Printf("LoRa init, please wait ...")

	must("SetCR:", radio.SetCR(SX127X.CR_5))
	must("SetBW:", radio.SetBW(SX127X.BW_125))
	must("SetSF:", radio.SetSF(SX127X.SF_12))
	must("SetSyncWord:", radio.SetSyncWord(0x12))
	must("SetChannel:", radio.SetChannel(SX127X.CH_06_868))
	radio.NeedPABOOST = true
	must("SetPowerDBM:", radio.SetPowerDBM(14))

	fmt.Println("LoRa successfully configured")

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Your name: ")
	name, _ := reader.ReadString('\n')

	output := make(chan string)

	go func() {
		for true {
			select {
			case line := <-output:
				radio.Write([]byte(line))
			default:
				time.Sleep(time.Millisecond * 100)
				line, err := radio.Read()
				if err != nil {
					fmt.Println("Error", err)
				}
				if len(line) != 0 {
					fmt.Println("<", line)
				}
			}
		}
	}()

	for true {
		fmt.Printf("> ")
		line, _ := reader.ReadString('\n')
		output <- fmt.Sprintf("[%5s] %s", name, line)
	}
}

func must(msg string, err error) {
	if err != nil {
		fmt.Printf("Error %s %v", msg, err)
		os.Exit(1)
	}
}
