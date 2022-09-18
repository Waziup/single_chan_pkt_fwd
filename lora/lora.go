package lora

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// TxPacket
type TxPacket struct {
	Immediate bool // Send packet immediately (will ignore tmst & time)

	CountUs uint32    // internal concentrator counter for timestamping, 1 microsecond resolution - Send packet on a certain timestamp value (will ignore time)
	TimeGPS time.Time // Send packet at a certain GPS time (GPS synchronization required)

	Freq uint32 // TX central frequency in Hz

	Power uint8 // TX output power in dBm

	ChainRF uint8 // Concentrator "RF chain" used for TX

	Modulation string // Modulation identifier "LORA" or "FSK"

	// LoRa only

	LoRaBW uint8 // LoRa bandwith: BW7K8 (0x01), BW10K4 (0x02), BW15K6 (0x03), BW20K8 (0x04), BW31K2 (0x05), BW41K7 (0x06), BW62K5 (0x07), BW125K (0x08), BW250K (0x09), BW500K (0x0a)
	LoRaCR uint8 // LoRa ECC coding rate: 4/5 (0x05), 4/6 (0x06), 4/7 (0x07), 4/8 (0x08)

	// LoRa only

	InvertPolar bool // Lora modulation polarization inversion

	// LoRa: LoRa spreading factor: SF7 (0x07) to SF12 (0x0c)
	// FSK: Datarate (bits per second)
	Datarate uint32

	PreambleLength uint16 // RF preamble size

	NoCRC bool // No CRC

	// FSK only
	FreqDev uint8 // FSK frequency deviation, in Hz

	Data []byte // packet payload
}

func (tx *TxPacket) UnmarshalJSON(data []byte) error {

	var txpk = struct {
		Immediate  bool        `json:"imme"` // "immediate" tag -> Class C
		CountUs    uint32      `json:"tmst"` // TX procedure: send on timestamp value -> Class A
		TimeGPS    uint64      `json:"tmms"` // GPS timestamp is given -> Class B
		NoCRC      bool        `json:"ncrc"` // "No CRC" flag (optional field)
		Freq       float64     `json:"freq"` // target frequency (mandatory)
		ChainRF    uint8       `json:"rfch"` // RF chain used for TX (mandatory)
		Power      uint8       `json:"powe"` // TX power (optional field)
		Modulation string      `json:"modu"` // modulation (mandatory)
		Datarate   interface{} `json:"datr"`
		// Datarate string `json:"datr"` // Lora spreading-factor and modulation bandwidth (mandatory) (LoRa only)
		// Datarate uint32 `json:"datr"` // FSK bitrate (mandatory) (FSK only)
		Coderate       string  `json:"codr"` // ECC coding rate (optional field)
		InvertPolar    bool    `json:"ipol"` // signal polarity switch (optional field)
		PreambleLength uint16  `json:"prea"` //  Lora/FSK preamble length (optional field)
		FreqDev        float32 `json:"fdev"` // frequency deviation in kHz (mandatory) (FSK only)
		Data           string  `json:"data"` // payload data (mandatory)
	}{}

	if err := json.Unmarshal(data, &txpk); err != nil {
		return err
	}

	tx.Immediate = txpk.Immediate
	tx.CountUs = txpk.CountUs
	tx.NoCRC = txpk.NoCRC
	tx.Freq = uint32(txpk.Freq * 1.0e6)
	tx.ChainRF = txpk.ChainRF
	tx.Power = txpk.Power
	switch txpk.Modulation {
	case "LORA":
		tx.Modulation = "LORA"
		var bw int

		datr, ok := txpk.Datarate.(string)
		if !ok {
			return fmt.Errorf("can not parse lora datarate (not a string): %+v", txpk.Datarate)
		}

		_, err := fmt.Sscanf(datr, "SF%dBW%d", &tx.Datarate, &bw)
		if err != nil {
			return fmt.Errorf("can not parse lora datarate %q: %v", datr, err)
		}
		switch bw {
		case 7:
			tx.LoRaBW = 1
		case 10:
			tx.LoRaBW = 2
		case 15:
			tx.LoRaBW = 3
		case 20:
			tx.LoRaBW = 4
		case 31:
			tx.LoRaBW = 5
		case 41:
			tx.LoRaBW = 6
		case 62:
			tx.LoRaBW = 7
		case 125:
			tx.LoRaBW = 8
		case 250:
			tx.LoRaBW = 9
		case 500:
			tx.LoRaBW = 10
		default:
			return fmt.Errorf("can not parse lora datarate %v: unknown bandwidth %d", datr, bw)
		}
		switch txpk.Coderate {
		case "4/5":
			tx.LoRaCR = 5
		case "4/6", "2/3":
			tx.LoRaCR = 6
		case "4/7":
			tx.LoRaCR = 7
		case "4/8", "2/4", "1/2":
			tx.LoRaCR = 8
		default:
			return fmt.Errorf("can not parse lora coderate: %q", txpk.Coderate)
		}
		tx.InvertPolar = txpk.InvertPolar
		tx.PreambleLength = txpk.PreambleLength
	case "FSK":
		tx.Modulation = "FSK"

		datr, ok := txpk.Datarate.(float64)
		if !ok {
			return fmt.Errorf("can not parse lora datarate (not a number): %+v", txpk.Datarate)
		}
		tx.Datarate = uint32(datr)

		tx.FreqDev = uint8(txpk.FreqDev / 1000.0)
		tx.PreambleLength = txpk.PreambleLength

	default:
		return fmt.Errorf("unknown modulation: %q", txpk.Modulation)
	}

	data, err := base64.StdEncoding.DecodeString(txpk.Data)
	if err != nil {
		return fmt.Errorf("can not decode data: %v", err)
	}
	tx.Data = data
	return nil
}

func (tx *TxPacket) String() string {
	data := base64.StdEncoding.EncodeToString(tx.Data)
	if tx.Modulation == "LORA" {
		versionMajor := tx.Data[0] & 0b11
		if versionMajor == LoRaWANR1 {
			mtype := MType(tx.Data[0] >> 5)
			devAddr := uint32(tx.Data[1])<<24 + uint32(tx.Data[2])<<16 + uint32(tx.Data[3])<<8 + uint32(tx.Data[4])
			fCnt := uint16(tx.Data[6])<<8 + uint16(tx.Data[7])
			return fmt.Sprintf("LoRaWAN %s: %.2f MHz, SF%d %s CR4/%d, Mote %08X, FCnt %d, Data: %s", mtype, float64(tx.Freq)/1e6, tx.Datarate, bwStr[tx.LoRaBW], tx.LoRaCR, devAddr, fCnt, data)
		}
		return fmt.Sprintf("LoRa: %.2f MHz, SF%d %s CR4/%d, Data: %s", float64(tx.Freq)/1e6, tx.Datarate, bwStr[tx.LoRaBW], tx.LoRaCR, data)
	}
	if tx.Modulation == "FSK" {
		return fmt.Sprintf("FSK: %.2f MHz, Bitrate %d, Data: %s", float64(tx.Freq)/1e6, tx.Datarate, data)
	}
	return "<unknown modulation>"
}

var bwStr = []string{
	"",
	"BW7.8",
	"BW10.4",
	"BW15.6",
	"BW20.8",
	"BW31.2",
	"BW41.7",
	"BW62.5",
	"BW125",
	"BW250",
	"BW500",
}

// RxPacket
type RxPacket struct {
	Time *time.Time // UTC time of pkt RX
	// TimeGPS time.Time // GPS time of pkt RX
	// TimeFin time.Time // Internal timestamp of "RX finished" event

	CountUs uint32 // internal concentrator counter for timestamping, 1 microsecond resolution

	Freq uint32 // RX central frequency in Hz

	ChainIF uint8 // Concentrator "IF" channel used for RX
	ChainRF uint8 // Concentrator "RF chain" used for RX or TX

	StatCRC int8 // CRC status: 1 = OK, -1 = fail, 0 = no CRC

	Modulation string // Modulation identifier "LORA" or "FSK"

	// LoRa only
	LoRaBW uint8 // LoRa bandwith: BW7K8 (0x01), BW10K4 (0x02), BW15K6 (0x03), BW20K8 (0x04), BW31K2 (0x05), BW41K7 (0x06), BW62K5 (0x07), BW125K (0x08), BW250K (0x09), BW500K (0x0a)
	LoRaCR uint8 // LoRa ECC coding rate: 4/5 (0x05), 4/6 (0x06), 4/7 (0x07), 4/8 (0x08)

	// LoRa: LoRa spreading factor: SF7 (0x07) to SF12 (0x0c)
	// FSK: Datarate (bits per second)
	Datarate uint32

	RSSI float32 // average packet RSSI in dB

	LoRaSNR float32 // average packet SNR, in dB

	Data []byte // packet payload
}

func (rx *RxPacket) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "{")
	if rx.Time != nil {
		fmt.Fprintf(&buf, "\"time\":\"%s\"", rx.Time.Format(time.RFC3339)) /* ISO 8601 format */
	} else {
		fmt.Fprintf(&buf, "\"tmst\":%d", rx.CountUs)
	}
	fmt.Fprintf(&buf, ",\"chan\":%d", rx.ChainIF)
	fmt.Fprintf(&buf, ",\"rfch\":%d", rx.ChainRF)
	fmt.Fprintf(&buf, ",\"freq\":%.3f", float64(rx.Freq)/1e6)
	fmt.Fprintf(&buf, ",\"stat\":%d", rx.StatCRC)
	if rx.Modulation == "LORA" {
		fmt.Fprint(&buf, ",\"modu\":\"LORA\"")
		fmt.Fprintf(&buf, ",\"datr\":\"SF%d%s\"", rx.Datarate, bwStr[rx.LoRaBW])
		fmt.Fprintf(&buf, ",\"codr\":\"4/%d\"", rx.LoRaCR)
		fmt.Fprintf(&buf, ",\"lsnr\":%.1f", rx.LoRaSNR)
	} else {
		fmt.Fprint(&buf, ",\"modu\":\"FSK\"")
		fmt.Fprintf(&buf, ",\"datr\":%d", rx.Datarate)
	}
	fmt.Fprintf(&buf, ",\"rssi\":%.0f", rx.RSSI)
	fmt.Fprintf(&buf, ",\"size\":%d", len(rx.Data))
	fmt.Fprintf(&buf, ",\"data\":\"%s\"}", base64.StdEncoding.EncodeToString(rx.Data))
	return buf.Bytes(), nil
}

const LoRaWANR1 = 0x00

type MType byte

var mTypeStr = []string{
	"Join Request",
	"Join Accept",
	"Unconfirmed Data Up",
	"Unconfirmed Data Down",
	"Confirmed Data Up",
	"Confirmed Data Down",
	"RFU",
	"Proprietary",
}

func (t MType) String() string {
	return mTypeStr[t]
}

func (rx *RxPacket) String() string {
	data := base64.StdEncoding.EncodeToString(rx.Data)
	if rx.Modulation == "LORA" {
		if len(rx.Data) > 8 {
			versionMajor := rx.Data[0] & 0b11
			if versionMajor == LoRaWANR1 {
				mtype := MType(rx.Data[0] >> 5)
				devAddr := uint32(rx.Data[4])<<24 + uint32(rx.Data[3])<<16 + uint32(rx.Data[2])<<8 + uint32(rx.Data[1])
				fCnt := uint16(rx.Data[7])<<8 + uint16(rx.Data[6])
				return fmt.Sprintf("LoRaWAN %s: %.2f MHz, SF%d %s CR4/%d, Mote %08X, FCnt %d, Data: %s", mtype, float64(rx.Freq)/1e6, rx.Datarate, bwStr[rx.LoRaBW], rx.LoRaCR, devAddr, fCnt, data)
			}
		}
		return fmt.Sprintf("LoRa: %.2f MHz, SF%d %s CR4/%d, Data: %s", float64(rx.Freq)/1e6, rx.Datarate, bwStr[rx.LoRaBW], rx.LoRaCR, data)
	}
	if rx.Modulation == "FSK" {
		return fmt.Sprintf("FSK: %.2f MHz, Bitrate %d, Data: %s", float64(rx.Freq)/1e6, rx.Datarate, data)
	}
	return ""
}

type Config struct {
	Freq uint32 `json:"freq"` // RX central frequency in Hz

	Modulation string `json:"modulation"` // Modulation identifier "LORA" or "FSK"

	// LoRa: Bandwidth 7800 .. 125000, 250000, 500000
	LoRaBW uint32 `json:"bandwidth"` // LoRa bandwidth
	// LoRa: Coderate 4/5 (0x05), 4/6 (0x06), 4/7 (0x07), 4/8 (0x08)
	LoRaCR string `json:"coderate"` // LoRa coderate

	// LoRa: LoRa spreading factor: SF7 (0x07) to SF12 (0x0c)
	// FSK: Datarate (bits per second)
	Datarate uint32 `json:"spread_factor"`

	PreambleLength uint16 // RF preamble size
}
