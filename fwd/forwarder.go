package fwd

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/Waziup/single_chan_pkt_fwd/lora"
)

type Stat struct {
}

type TxAckError int

const (
	NoError            TxAckError = iota + 1 // Packet has been programmed for downlink
	ErrTooLate                               // Rejected because it was already too late to program this packet for downlink
	ErrTooEarly                              // Rejected because downlink packet timestamp is too much in advance
	ErrCollisionPacket                       // Rejected because there was already a packet programmed in requested timeframe
	ErrCollisionBeacon                       // Rejected because there was already a beacon planned in requested timeframe
	ErrTxFreq                                // Rejected because requested frequency is not supported by TX RF chain
	ErrTxPower                               // Rejected because requested power is not supported by gateway
	ErrGPSUnloacked                          // Rejected because GPS is unlocked, so GPS timestamp cannot be used
)

func (err TxAckError) MarshalJSON() ([]byte, error) {
	var errStr = []string{
		"",
		"NONE",
		"TOO_LATE",
		"TOO_EARLY",
		"COLLISION_PACKET",
		"COLLISION_BEACON",
		"TX_FREQ",
		"TX_POWER",
		"GPS_UNLOCKED",
	}
	return []byte(fmt.Sprintf("{\"error\":\"%s\"}", errStr[err])), nil
}

func (err TxAckError) Error() string {
	var errStr = []string{
		"",
		"",
		"too late to program this packet for downlink",
		"timestamp is too much in advance",
		"there was already a packet programmed in requested timeframe",
		" there was already a beacon planned in requested timeframe",
		"requested frequency is not supported by TX RF chain",
		" requested power is not supported by gateway",
		"GPS is unlocked, so GPS timestamp cannot be used",
	}
	return errStr[err]
}

type TxAckMsg struct {
	Error TxAckError `json:"error"`
}

type Ident uint8

type Token [2]byte

func (t Token) String() string {
	return fmt.Sprintf("%X", int(t[1])<<8+int(t[0]))
}

type Packet struct {
	Token     Token            `json:"-"`
	Ident     Ident            `json:"-"`
	GatewayID uint64           `json:"-"`
	Stat      *Stat            `json:"stat,omitempty"`
	RxPackets []*lora.RxPacket `json:"rxpk,omitempty"`
	TxPacket  *lora.TxPacket   `json:"txpk,omitempty"`
	TxAck     TxAckError       `json:"txpk_ack,omitempty"`
}

func (pkt *Packet) String() string {
	switch pkt.Ident {
	case PullData:
		return fmt.Sprintf("%s: Token: %s, Gateway ID: %X", pkt.Ident, pkt.Token, pkt.GatewayID)
	case PullAck:
		return fmt.Sprintf("%s: Token: %s", pkt.Ident, pkt.Token)
	case PushData:
		return fmt.Sprintf("%s: Token: %s, Gateway ID: %X, %d rx packets", pkt.Ident, pkt.Token, pkt.GatewayID, len(pkt.RxPackets))
	case PushAck:
		return fmt.Sprintf("%s: Token: %s", pkt.Ident, pkt.Token)
	case PullResp:
		return fmt.Sprintf("%s: Token: %s, 1 tx packet", pkt.Ident, pkt.Token)
	case TxAck:
		return fmt.Sprintf("%s: Token: %s, Gateway ID: %X", pkt.Ident, pkt.Token, pkt.GatewayID)
	}
	return "(unknwon packet)"
}

func (i Ident) String() string {
	var iStr = []string{
		"PushData",
		"PushAck",
		"PullData",
		"PullResp",
		"PullAck",
		"TxAck",
	}
	if i < 0 || int(i) >= len(iStr) {
		return "(unknown)"
	}
	return iStr[i]
}

const (
	PushData Ident = 0x00
	PushAck        = 0x01
	PullData       = 0x02
	PullResp       = 0x03
	PullAck        = 0x04
	TxAck          = 0x05
)

func RndToken() [2]byte {
	var token [2]byte
	rand.Read(token[:])
	return token
}

func (p *Packet) MarshalBinary() ([]byte, error) {

	var buf bytes.Buffer
	buf.WriteByte(0x02)   // protocol version = 2
	buf.Write(p.Token[:]) // random token

	switch p.Ident {
	case PushData:
		buf.WriteByte(byte(PushData))                     // PUSH_DATA identifier 0x00
		binary.Write(&buf, binary.BigEndian, p.GatewayID) // Gateway unique identifier (MAC address)
		// rand.Read(token[:])
		encoder := json.NewEncoder(&buf)
		err := encoder.Encode(p)
		return buf.Bytes(), err
	case PullData:
		buf.WriteByte(byte(PullData))                     // PULL_DATA identifier 0x02
		binary.Write(&buf, binary.BigEndian, p.GatewayID) // Gateway unique identifier (MAC address)
		return buf.Bytes(), nil
	case TxAck:
		buf.WriteByte(byte(TxAck))                        // TX_ACK identifier 0x05
		binary.Write(&buf, binary.BigEndian, p.GatewayID) // Gateway unique identifier (MAC address)
		return buf.Bytes(), nil

	default:
		return nil, fmt.Errorf("unknown packet type: %d", p.Ident)
	}
}

func (p *Packet) UnmarshalBinary(buf []byte) error {

	if len(buf) < 4 {
		return fmt.Errorf("buffer to short")
	}
	if buf[0] != 0x02 {
		return fmt.Errorf("can not handle version: 0x%x", buf[0])
	}
	copy(p.Token[:], buf[1:3])
	p.Ident = Ident(buf[3])
	switch p.Ident {
	case PushAck, PullAck:
		return nil
	case PullResp:
		err := json.Unmarshal(buf[4:], p)
		if err != nil {
			return fmt.Errorf("can not unmarshal PULL_RESP packet: %q", err)
		}
		return nil
	default:
		return fmt.Errorf("can not unmarshal downstream packet type 0x%x", buf[3])
	}
}
