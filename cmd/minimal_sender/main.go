package main

import (
	"log"
	"time"

	"github.com/Waziup/single_chan_pkt_fwd/SX127X"
	"github.com/Waziup/single_chan_pkt_fwd/lora"
)

func main() {
	log.SetFlags(0)
	// SX127X.LogLevel = SX127X.LogLevelDebug

	radio, err := SX127X.Discover()
	if err != nil {
		log.Fatalf("can not activate radio: %v", err)
	}

	log.Printf("SX1272 init ...")

	// must("SetIQInversion:", radio.SetIQInversion(true))

	must("SetCR:", radio.SetCR(SX127X.CR_5))
	must("SetBW:", radio.SetBW(SX127X.BW_125))
	must("SetSF:", radio.SetSF(SX127X.SF_12))
	must("SetSyncWord:", radio.SetSyncWord(0x34))
	must("SetChannel:", radio.SetChannel(SX127X.CH_18_868))

	radio.NeedPABOOST = true
	must("SetPowerDBM:", radio.SetPowerDBM(14))

	log.Printf("SX1272 successfully configured")

	msg := []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7}
	start := time.Now()
	must("Send:", radio.Send(&lora.TxPacket{
		InvertPolar: true,
		Modulation:  "LORA",
		LoRaCR:      5,
		LoRaBW:      0x08,
		Freq:        868100000,
		Datarate:    12,
		Power:       14,
		Data:        msg,
	}))
	// must("Write:", radio.Write(msg))
	end := time.Now()
	log.Printf("LoRa Sent in %s", end.Sub(start))
}

func must(msg string, err error) {
	if err != nil {
		log.Fatalf("%s %v", msg, err)
	} else {
		log.Printf("%s OK", msg)
	}
}
