package SX127X

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Waziup/single_chan_pkt_fwd/lora"
	"github.com/Waziup/wazigate-rpi/gpio"
	"github.com/Waziup/wazigate-rpi/spi"
)

const (
	RegVersion  = 0x42
	RegPaConfig = 0x09
)

const High = true
const Low = false

const VersionSX1272 = 0x22
const VersionSX1276 = 0x12

const ModeLoRa = 1
const ModemFSK = 0

const LogLevelNone = 0
const LogLevelDebug = 5
const LogLevelVerbose = 4
const LogLevelNormal = 3
const LogLevelWarning = 2
const LogLevelError = 1

type Chip struct {
	pinSS           gpio.Pin
	pinRst          gpio.Pin
	dev             *spi.Device
	version         byte
	defaultSyncWord byte

	LogLevel int
	Logger   *log.Logger

	mode            int
	syncWord        byte
	spreadingFactor uint32
	retries         int
	maxRetries      int
	payloadLength   byte
	codingRate      byte
	bandwidth       byte
	header          bool
	nodeAddress     byte
	needPABOOST     bool
	power           byte
	channel         uint32
}

var logLevel = []string{
	"[     ] ",
	"[ERR  ] ",
	"[WARN ] ",
	"",
	"[VERBO] ",
	"[DEBUG] ",
}

var LogLevel = LogLevelNone
var Logger *log.Logger

const (
	PrivateSyncWord = 0x12
	PublicSyncWord  = 0x34
)

func New(dev *spi.Device, pinSS gpio.Pin, pinRst gpio.Pin) *Chip {
	return &Chip{
		dev:             dev,
		pinSS:           pinSS,
		pinRst:          pinRst,
		defaultSyncWord: PublicSyncWord,
		spreadingFactor: 0,
		needPABOOST:     true,
		nodeAddress:     1,
	}
}

func (c *Chip) Log(level int, format string, v ...interface{}) {
	if level <= c.LogLevel && level >= 0 && level < 6 {
		c.Logger.Printf(logLevel[level]+format, v...)
	}
}

func (c *Chip) readRegister(addr byte) (byte, error) {
	c.pinSS.Write(Low)
	buf := [2]byte{addr & 0x7F, 0}
	err := c.dev.Transfer(buf[0:])
	c.pinSS.Write(High)
	return buf[1], err
}

func (c *Chip) writeRegister(addr byte, data byte) error {
	c.pinSS.Write(Low)
	buf := [2]byte{addr | 0x80, data}
	err := c.dev.Transfer(buf[0:])
	c.pinSS.Write(High)
	return err
}

func Discover() (*Chip, error) {

	dev, err := spi.Open("/dev/spidev0.1", 1000000)
	if err != nil {
		return nil, err
	}
	dev.SetMode(0)
	dev.SetBitsPerWord(8)
	dev.SetLSBFirst(false)
	dev.SetMaxSpeed(1000000)

	// SlaveSelect GPIO Pin
	pinSS, err := gpio.Output(8)
	if err != nil {
		return nil, err
	}

	// Reset GIOP Pin
	pinRst, err := gpio.Output(4)
	if err != nil {
		return nil, err
	}

	// SX127X instance
	sx := New(dev, pinSS, pinRst)
	sx.Logger = Logger
	sx.LogLevel = LogLevel

	sx.pinSS.Write(High)
	delay(100)

	sx.pinRst.Write(Low)
	delay(100)
	sx.pinRst.Write(High)
	delay(100)

	version, err := sx.readRegister(RegVersion)
	if err != nil {
		sx.Close()
		return nil, err
	}
	sx.version = version
	if version != VersionSX1272 && version != VersionSX1276 {
		sx.Close()
		return nil, fmt.Errorf("unknown chip version: 0x%x", version)
	}

	return sx, nil
}

func (c *Chip) Name() string {
	switch c.version {
	case VersionSX1272:
		return "SX1272"
	case VersionSX1276:
		return "SX1276"
	}
	return ""
}

// On Sets the module ON.
func (c *Chip) Init() error {

	c.Log(LogLevelDebug, "Starting 'ON'.")

	if err := c.RxChainCalibration(); err != nil {
		return err
	}
	if err := c.SetMaxCurrent(0x1B); err != nil {
		return err
	}
	c.Log(LogLevelVerbose, "Setting ON with maximum current supply.")

	if err := c.SetLORA(); err != nil {
		return err
	}

	preambleLength, err := c.GetPreambleLength()
	if err != nil {
		return err
	}

	c.Log(LogLevelVerbose, "Preamble length: %d.", preambleLength)

	// CAUTION
	// doing initialization as proposed by Libelium seems not to work for the SX1276
	// so we decided to leave the default value of the SX127x, then configure the radio when
	// setting to LoRa mode

	//Set initialization values

	c.writeRegister(0x0, 0x0)
	// comment by C. Pham
	// still valid for SX1276
	c.writeRegister(0x1, 0x81)
	// end
	c.writeRegister(0x2, 0x1A)
	c.writeRegister(0x3, 0xB)
	c.writeRegister(0x4, 0x0)
	c.writeRegister(0x5, 0x52)
	c.writeRegister(0x6, 0xD8)
	c.writeRegister(0x7, 0x99)
	c.writeRegister(0x8, 0x99)
	// modified by C. Pham
	// added by C. Pham
	if c.mode == VersionSX1272 {
		// RFIO_pin RFU OutputPower
		// 0 000 0000
		c.writeRegister(0x9, 0x0)
	} else {
		// RFO_pin MaxP OutputPower
		// 0 100 1111
		// set MaxPower to 0x4 and OutputPower to 0
		c.writeRegister(0x9, 0x40)
	}

	c.writeRegister(0xA, 0x9)
	c.writeRegister(0xB, 0x3B)

	// comment by C. Pham
	// still valid for SX1276
	c.writeRegister(0xC, 0x23)

	// REG_RX_CONFIG
	c.writeRegister(0xD, 0x1)

	c.writeRegister(0xE, 0x80)
	c.writeRegister(0xF, 0x0)
	c.writeRegister(0x10, 0x0)
	c.writeRegister(0x11, 0x0)
	c.writeRegister(0x12, 0x0)
	c.writeRegister(0x13, 0x0)
	c.writeRegister(0x14, 0x0)
	c.writeRegister(0x15, 0x0)
	c.writeRegister(0x16, 0x0)
	c.writeRegister(0x17, 0x0)
	c.writeRegister(0x18, 0x10)
	c.writeRegister(0x19, 0x0)
	c.writeRegister(0x1A, 0x0)
	c.writeRegister(0x1B, 0x0)
	c.writeRegister(0x1C, 0x0)

	// added by C. Pham
	if c.mode == VersionSX1272 {
		// comment by C. Pham
		// 0x4A = 01 001 0 1 0
		// BW=250 CR=4/5 ImplicitH_off RxPayloadCrcOn_on LowDataRateOptimize_off
		c.writeRegister(0x1D, 0x4A)
		// 1001 0 1 11
		// SF=9 TxContinuous_off AgcAutoOn SymbTimeOut
		c.writeRegister(0x1E, 0x97)
	} else {
		// 1000 001 0
		// BW=250 CR=4/5 ImplicitH_off
		c.writeRegister(0x1D, 0x82)
		// 1000 0 1 11
		// SF=9 TxContinuous_off RxPayloadCrcOn_on SymbTimeOut
		c.writeRegister(0x1E, 0x97)
	}
	// end

	c.writeRegister(0x1F, 0xFF)
	c.writeRegister(0x20, 0x0)
	c.writeRegister(0x21, 0x8)
	c.writeRegister(0x22, 0xFF)
	c.writeRegister(0x23, 0xFF)
	c.writeRegister(0x24, 0x0)
	c.writeRegister(0x25, 0x0)

	// added by C. Pham
	if c.mode == VersionSX1272 {
		c.writeRegister(0x26, 0x0)
	} else {
		// 0000 0 1 00
		// reserved LowDataRateOptimize_off AgcAutoOn reserved
		c.writeRegister(0x26, 0x04)
	}

	// REG_SYNC_CONFIG
	c.writeRegister(0x27, 0x0)

	c.writeRegister(0x28, 0x0)
	c.writeRegister(0x29, 0x0)
	c.writeRegister(0x2A, 0x0)
	c.writeRegister(0x2B, 0x0)
	c.writeRegister(0x2C, 0x0)
	c.writeRegister(0x2D, 0x50)
	c.writeRegister(0x2E, 0x14)
	c.writeRegister(0x2F, 0x40)
	c.writeRegister(0x30, 0x0)
	c.writeRegister(0x31, 0x3)
	c.writeRegister(0x32, 0x5)
	c.writeRegister(0x33, 0x27)
	c.writeRegister(0x34, 0x1C)
	c.writeRegister(0x35, 0xA)
	c.writeRegister(0x36, 0x0)
	c.writeRegister(0x37, 0xA)
	c.writeRegister(0x38, 0x42)
	c.writeRegister(0x39, 0x12)
	c.writeRegister(0x3A, 0x65)
	c.writeRegister(0x3B, 0x1D)
	c.writeRegister(0x3C, 0x1)
	c.writeRegister(0x3D, 0xA1)
	c.writeRegister(0x3E, 0x0)
	c.writeRegister(0x3F, 0x0)
	c.writeRegister(0x40, 0x0)
	c.writeRegister(0x41, 0x0)

	c.SetSyncWord(c.defaultSyncWord)

	/*
		if err := c.SetMode(1); err != nil {
			return err
		}
		if err := c.SetChannel(CH_10_868); err != nil {
			return err
		}
		if err := c.SetPowerDBM(14); err != nil {
			return err
		}

		c.Logger.SetOutput(os.Stdout)
		c.LogLevel = LogLevelDebug
		for true {
		 	data, err := c.ReceiveAll(10000)
		 	log.Println(data, err)
		}
	*/

	return nil
}

func (c *Chip) Close() error {
	c.dev.Close()
	c.pinSS.Write(Low)
	c.pinSS.Unexport()
	c.pinRst.Unexport()
	return nil
}

func delay(d int) {
	time.Sleep((time.Millisecond * time.Duration(d)))
}

func (c *Chip) SetSyncWord(sw byte) (err error) {

	st0, _ := c.readRegister(REG_OP_MODE) // Save the previous status

	if c.mode == ModemFSK {
		c.Log(LogLevelWarning, "Notice that FSK hasn't sync word parameter, so you are configuring it in LoRa mode.")
		c.SetLORA()
	}
	c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE) // Set Standby mode to write in registers

	c.writeRegister(REG_SYNC_WORD, sw)

	delay(100)

	config1, _ := c.readRegister(REG_SYNC_WORD)

	if config1 == sw {
		c.syncWord = sw
		c.Log(LogLevelVerbose, "Sync Word 0x%x has been successfully set.", sw)
	} else {
		c.Log(LogLevelError, "There has been an error while configuring Sync Word parameter.")
		err = fmt.Errorf("can not set sync word: got 0x%x, expected 0x%x", config1, sw)
	}

	c.writeRegister(REG_OP_MODE, st0) // Getting back to previous status
	delay(100)
	return
}

func (c *Chip) RxChainCalibration() (err error) {
	var v byte
	if c.version == VersionSX1276 {

		c.Log(LogLevelVerbose, "SX1276 LF/HF calibration ...")
		// Cut the PA just in case, RFO output, power = -1 dBm
		c.writeRegister(REG_PA_CONFIG, 0x00)

		// Launch Rx chain calibration for LF band
		v, _ = c.readRegister(REG_IMAGE_CAL)
		c.writeRegister(REG_IMAGE_CAL, (v&RF_IMAGECAL_IMAGECAL_MASK)|RF_IMAGECAL_IMAGECAL_START)
		for v, _ = c.readRegister(REG_IMAGE_CAL); (v & RF_IMAGECAL_IMAGECAL_RUNNING) == RF_IMAGECAL_IMAGECAL_RUNNING; {
			v, _ = c.readRegister(REG_IMAGE_CAL)
		}

		err = c.SetChannel(CH_17_868)

		// Launch Rx chain calibration for HF band
		v, _ = c.readRegister(REG_IMAGE_CAL)
		c.writeRegister(REG_IMAGE_CAL, (v&RF_IMAGECAL_IMAGECAL_MASK)|RF_IMAGECAL_IMAGECAL_START)
		for v, _ = c.readRegister(REG_IMAGE_CAL); (v & RF_IMAGECAL_IMAGECAL_RUNNING) == RF_IMAGECAL_IMAGECAL_RUNNING; {
			v, _ = c.readRegister(REG_IMAGE_CAL)
		}
	}
	return
}

func (c *Chip) SetFreq(freq uint32) (err error) {
	var ch = uint32((uint64(freq) << 19) / 32000000)
	return c.SetChannel(ch)
}

func (c *Chip) GetFreq() uint32 {
	return uint32((uint64(c.channel) * 32000000) >> 19)
}

func (c *Chip) SetChannel(ch uint32) (err error) {

	st0, _ := c.readRegister(REG_OP_MODE) // Save the previous status
	if c.mode == ModeLoRa {
		c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE)
	} else {
		c.writeRegister(REG_OP_MODE, FSK_STANDBY_MODE)
	}
	/*
		if( _modem == LORA )
		{
			// LoRa Stdby mode in order to write in registers
			writeRegister(REG_OP_MODE, LORA_STANDBY_MODE);
		}
		else
		{
			// FSK Stdby mode in order to write in registers
			writeRegister(REG_OP_MODE, FSK_STANDBY_MODE);
		}
	*/

	frf3 := byte((ch >> 16) & 0x0FF) // frequency channel MSB
	frf2 := byte((ch >> 8) & 0x0FF)  // frequency channel MIB
	frf1 := byte(ch & 0xFF)          // frequency channel LSB

	// storing MSB in freq channel value
	c.writeRegister(REG_FRF_MSB, frf3)
	// storing MID in freq channel value
	c.writeRegister(REG_FRF_MID, frf2)
	// storing LSB in freq channel value
	c.writeRegister(REG_FRF_LSB, frf1)

	time.Sleep(100 * time.Millisecond)

	frf3, _ = c.readRegister(REG_FRF_MSB)
	frf2, _ = c.readRegister(REG_FRF_MID)
	frf1, _ = c.readRegister(REG_FRF_LSB)

	frf := uint32(frf3)<<16 + uint32(frf2)<<8 + uint32(frf1)

	if frf != ch {
		err = fmt.Errorf("can not change channel: got %x, expected %x", frf, ch)
	} else {
		var freq = uint32((uint64(ch) * 32000000) >> 19)
		c.Log(LogLevelVerbose, "Channel changed to 0x%06x: %.2f Mhz", ch, float64(freq)/1e6)
		c.channel = ch
	}

	c.writeRegister(REG_OP_MODE, st0) // Getting back to previous status
	time.Sleep(100 * time.Millisecond)
	return
}

func (c *Chip) SetMaxCurrent(rate byte) error {

	// Maximum rate value = 0x1B, because maximum current supply = 240 mA
	if rate > 0x1B {
		return fmt.Errorf("maximum supply 0x1B exceeded: %x", rate)
	}

	// Enable Over Current Protection
	rate |= 0x20
	st0, _ := c.readRegister(REG_OP_MODE) // Save the previous status
	if c.mode == ModeLoRa {
		c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE)
	} else {
		c.writeRegister(REG_OP_MODE, FSK_STANDBY_MODE)
	}
	c.writeRegister(REG_OCP, rate)    // Modifying maximum current supply
	c.writeRegister(REG_OP_MODE, st0) // Getting back to previous status
	return nil
}

func (c *Chip) GetPreambleLength() (length int, err error) {

	if c.mode == ModeLoRa {
		// LORA mode
		l0, _ := c.readRegister(REG_PREAMBLE_MSB_LORA)
		l1, _ := c.readRegister(REG_PREAMBLE_LSB_LORA)
		return int(l0)<<8 + int(l1), nil
	}
	// FSK mode
	l0, _ := c.readRegister(REG_PREAMBLE_MSB_FSK)
	l1, _ := c.readRegister(REG_PREAMBLE_LSB_FSK)
	return int(l0)<<8 + int(l1), nil
}

func (c *Chip) SetLORA() error {

	// modified by C. Pham
	retry := 0
	var st0 byte

	for st0 != LORA_STANDBY_MODE {
		time.Sleep(time.Millisecond * 200)
		c.writeRegister(REG_OP_MODE, FSK_SLEEP_MODE)  // Sleep mode (mandatory to set LoRa mode)
		c.writeRegister(REG_OP_MODE, LORA_SLEEP_MODE) // LoRa sleep mode
		c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE)
		time.Sleep(time.Millisecond * time.Duration(50+10*retry))
		st0, _ = c.readRegister(REG_OP_MODE)

		if (retry % 2) == 0 {
			if retry == 20 {
				retry = 0
			} else {
				retry++
			}
		}
	}

	if st0 == LORA_STANDBY_MODE {
		// LoRa mode
		c.mode = ModeLoRa
		return nil
	}

	// LoRa mode
	c.mode = ModeLoRa
	return fmt.Errorf("could not enable LoRa mode")
}

func (c *Chip) receive() (err error) {

	c.Log(LogLevelDebug, "Starting 'receive'.")

	// Setting Testmode
	// commented by C. Pham
	// writeRegister(0x31,0x43)

	// Set LowPnTxPllOff
	// modified by C. Pham from 0x09 to 0x08
	c.writeRegister(REG_PA_RAMP, 0x08)

	// writeRegister(REG_LNA, 0x23)			// Important in reception
	// modified by C. Pham
	c.writeRegister(REG_LNA, LNA_MAX_GAIN)
	c.writeRegister(REG_FIFO_ADDR_PTR, 0x00) // Setting address pointer in FIFO data buffer
	// change RegSymbTimeoutLsb
	// comment by C. Pham
	// single_chan_pkt_fwd uses 00 00001000
	// why here we have 11 11111111
	// change RegSymbTimeoutLsb
	// writeRegister(REG_SYMB_TIMEOUT_LSB, 0xFF)

	// modified by C. Pham
	if c.spreadingFactor == SF_10 || c.spreadingFactor == SF_11 || c.spreadingFactor == SF_12 {
		c.writeRegister(REG_SYMB_TIMEOUT_LSB, 0x05)
	} else {
		c.writeRegister(REG_SYMB_TIMEOUT_LSB, 0x08)
	}
	//end

	c.writeRegister(REG_FIFO_RX_BYTE_ADDR, 0x00) // Setting current value of reception buffer pointer

	//clearFlags();						// Initializing flags

	//state = 1;
	if c.mode == ModeLoRa {
		// LoRa mode
		c.setPacketLength(MAX_LENGTH)              // With MAX_LENGTH gets all packets with length < MAX_LENGTH
		c.writeRegister(REG_OP_MODE, LORA_RX_MODE) // LORA mode - Rx
		c.Log(LogLevelDebug, "Receiving LoRa mode activated with success.")
	} else {
		// FSK mode
		c.setPacketLength(c.payloadLength)
		c.writeRegister(REG_OP_MODE, FSK_RX_MODE) // FSK mode - Rx
		c.Log(LogLevelDebug, "Receiving FSK mode activated with succes.")
	}
	return nil
}

func (c *Chip) setPacketLength(length byte) (err error) {

	c.Log(LogLevelDebug, "Starting 'setPacketLength'.")

	st0, _ := c.readRegister(REG_OP_MODE) // Save the previous status

	// packet_sent.length = length

	var l byte
	if c.mode == ModeLoRa { // LORA mode
		c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE) // Set LoRa Standby mode to write in registers
		c.writeRegister(REG_PAYLOAD_LENGTH_LORA, length)
		// Storing payload length in LoRa mode
		l, _ = c.readRegister(REG_PAYLOAD_LENGTH_LORA)
	} else {
		// FSK mode
		c.writeRegister(REG_OP_MODE, FSK_STANDBY_MODE) //  Set FSK Standby mode to write in registers
		c.writeRegister(REG_PAYLOAD_LENGTH_FSK, length)
		// Storing payload length in FSK mode
		l, _ = c.readRegister(REG_PAYLOAD_LENGTH_FSK)
	}

	if l == length {
		c.Log(LogLevelDebug, "Packet length %d has been successfully set.", length)
	} else {
		c.Log(LogLevelError, "Can not set packet length: expected %d, got %d", length, l)
		err = fmt.Errorf("can not set packet length")
	}

	c.writeRegister(REG_OP_MODE, st0) // Getting back to previous status
	// comment by C. Pham
	// this delay is included in the send delay overhead
	// TODO: do we really need this delay?
	delay(250)
	return
}

var ErrIncorrectCRC = fmt.Errorf("incorrect CRC")
var ErrTimeout = fmt.Errorf("timeout")

func (c *Chip) Receive(cfg *lora.Config) error {

	if err := c.SetFreq(cfg.Freq); err != nil {
		return err
	}

	c.SetPowerDBM(14)

	var bw byte
	switch cfg.LoRaBW {
	case 7800:
		bw = BW_7_8
	case 10400:
		bw = BW_10_4
	case 15600:
		bw = BW_15_6
	case 20800:
		bw = BW_20_8
	case 31250:
		bw = BW_31_25
	case 41700:
		bw = BW_41_7
	case 62500:
		bw = BW_62_5
	case 125000:
		bw = BW_125
	case 250000:
		bw = BW_250
	case 500000:
		bw = BW_500
	default:
		return fmt.Errorf("unknown bandwidth: %d", cfg.LoRaBW)
	}

	c.setCR(CR_5)
	c.setBW(bw)
	c.setSF(cfg.Datarate)

	// c.bandwidth = bw
	// c.spreadingFactor = cfg.Datarate
	// if err := c.setBW(bw); err != nil {
	// 	return err
	// }
	// if err := c.setSF(cfg.Datarate); err != nil {
	// 	return err
	// }

	// from ReceiveAll()
	if c.mode == ModemFSK { // FSK mode
		c.writeRegister(REG_OP_MODE, FSK_STANDBY_MODE) // Setting standby FSK mode
		config1, _ := c.readRegister(REG_PACKET_CONFIG1)
		config1 &= 0xf9                              // (0B11111001) clears bits 2-1 from REG_PACKET_CONFIG1
		c.writeRegister(REG_PACKET_CONFIG1, config1) // AddressFiltering = None
	}

	return c.receive()
}

func (c *Chip) GetPacket() ([]*lora.RxPacket, error) {
	data, crc, err := c.getPacket()
	if data == nil || err != nil {
		return nil, err
	}
	rssi, _ := c.GetRSSIpacket()
	snr, _ := c.getSNR()
	pkt := &lora.RxPacket{
		Time:       time.Now(),
		RSSI:       float32(rssi),
		Data:       data,
		StatCRC:    crc,
		Freq:       c.GetFreq(),
		Modulation: "LORA",
		Datarate:   uint32(c.spreadingFactor),
		LoRaCR:     c.codingRate + 4,
		LoRaBW:     c.bandwidth + 1,
		LoRaSNR:    float32(snr),
	}
	return []*lora.RxPacket{pkt}, err
}

func (c *Chip) getPacket() (data []byte, crc int8, err error) {

	c.Log(LogLevelDebug, "Starting 'getPacket'.")

	//previous = millis();
	// exitTime = millis() + (unsigned long)wait;

	if c.mode == ModeLoRa {
		// LoRa mode
		value, _ := c.readRegister(REG_IRQ_FLAGS)
		// Wait until the packet is received (RxDone flag) or the timeout expires
		//while( (bitRead(value, 6) == 0) && (millis() - previous < (unsigned long)wait) )
		if value&Bit6 == 0 {
			return nil, 0, nil
		}

		// modified by C. Pham

		//CrcOnPayload?
		v, _ := c.readRegister(REG_HOP_CHANNEL)
		if v&Bit6 != 0 {

			if value&Bit5 == 0 {
				// packet received & CRC correct
				crc = 1
				c.Log(LogLevelDebug, "CRC is correct.")
			} else {
				crc = -1
				c.Log(LogLevelDebug, "CRC is incorrect!")
			}
		} else {
			// as CRC is not set we suppose that CRC is correct
			crc = 0
			c.Log(LogLevelDebug, "Packet supposed to be correct as CrcOnPayload is off at transmitter.")
		}
		c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE) // Setting standby LoRa mode
	} else {

		// FSK mode
		value, _ := c.readRegister(REG_IRQ_FLAGS2)
		//while( (bitRead(value, 2) == 0) && (millis() - previous < wait) )
		if value&Bit2 == 0 {
			return nil, 0, nil
		}

		// packet received
		if value&Bit1 != 0 {
			// CRC correct
			crc = 1
			c.Log(LogLevelDebug, "Packet correctly received in FSK mode.")
		} else {
			// CRC incorrect
			crc = -1
			c.Log(LogLevelDebug, "Packet incorrectly received in FSK mode.")
		}

		c.writeRegister(REG_OP_MODE, FSK_STANDBY_MODE) // Setting standby FSK mode
	}

	// Store the packet
	if c.mode == ModeLoRa {
		// comment by C. Pham
		// set the FIFO addr to 0 to read again the destination
		c.writeRegister(REG_FIFO_ADDR_PTR, 0x00) // Setting address pointer in FIFO data buffer

		// added by C. Pham
		// packet_received.netkey[0]=readRegister(REG_FIFO);
		// packet_received.netkey[1]=readRegister(REG_FIFO);

	} else {
		value, _ := c.readRegister(REG_PACKET_CONFIG1)
		if value&Bit2 == 0 && value&Bit1 == 0 {
			// Storing first byte of the received packet
			_, _ = c.readRegister(REG_FIFO)
		} else {

			// dest = DefaultDestination
		}
	}

	length, _ := c.readRegister(REG_RX_NB_BYTES)
	data = make([]byte, length)
	for i := 0; i < int(length); i++ {
		data[i], _ = c.readRegister(REG_FIFO) // Storing payload
	}
	c.Log(LogLevelDebug, "Received: %v", data)

	if c.mode == ModeLoRa {
		c.writeRegister(REG_FIFO_ADDR_PTR, 0x00) // Setting address pointer in FIFO data buffer
	}

	c.clearFlags() // Initializing flags
	return
}

func (c *Chip) clearFlags() {

	st0, _ := c.readRegister(REG_OP_MODE) // Save the previous status

	if c.mode == ModeLoRa {
		// LoRa mode
		c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE) // Stdby mode to write in registers
		c.writeRegister(REG_IRQ_FLAGS, 0xFF)            // LoRa mode flags register
		c.writeRegister(REG_OP_MODE, st0)               // Getting back to previous status
		c.Log(LogLevelDebug, "LoRa flags cleared.")
	} else {
		// FSK mode
		c.writeRegister(REG_OP_MODE, FSK_STANDBY_MODE) // Stdby mode to write in registers
		c.writeRegister(REG_IRQ_FLAGS1, 0xFF)          // FSK mode flags1 register
		c.writeRegister(REG_IRQ_FLAGS2, 0xFF)          // FSK mode flags2 register
		c.writeRegister(REG_OP_MODE, st0)              // Getting back to previous status
		c.Log(LogLevelDebug, "FSK flags cleared.")
	}
}

func (c *Chip) SetMode(m int) (err error) {

	c.Log(LogLevelDebug, "Change mode to %d ...", m)

	if m < 0 || m >= len(modes) {
		return fmt.Errorf("there is no mode %d", m)
	}
	mode := modes[m]
	if c.codingRate != mode.cr {
		if err = c.setCR(mode.cr); err != nil {
			return err
		}
	}
	if c.spreadingFactor != uint32(mode.sf) {
		if err = c.setSF(uint32(mode.sf)); err != nil {
			return err
		}
	}
	if c.bandwidth != mode.bw {
		if err = c.setBW(mode.bw); err != nil {
			return err
		}
	}
	return nil
}

func (c *Chip) getSNR() (snr int8, err error) {

	c.Log(LogLevelDebug, "Starting 'getSNR'.")

	if c.mode == ModeLoRa {
		// LoRa mode
		value, _ := c.readRegister(REG_PKT_SNR_VALUE)

		if value&0x80 != 0 {
			// The SNR sign bit is 1
			// Invert and divide by 4
			value = ((0xff ^ value + 1) & 0xFF) >> 2
			snr = -int8(value)
		} else {
			// Divide by 4
			snr = int8((value & 0xFF) >> 2)
		}

		c.Log(LogLevelVerbose, "SNR value is %d.", snr)
	} else {
		// forbidden command if FSK mode
		c.Log(LogLevelWarning, "SNR does not exist in FSK mode.")
	}
	return
}

func (c *Chip) GetRSSIpacket() (rssi int16, err error) {
	// RSSIpacket only exists in LoRa

	c.Log(LogLevelDebug, "Starting 'getRSSIpacket'.")

	if c.mode == ModeLoRa {
		// LoRa mode

		snr, _ := c.getSNR()

		// added by C. Pham
		rssiv, _ := c.readRegister(REG_PKT_RSSI_VALUE)

		if snr < 0 {
			// commented by C. Pham
			//_RSSIpacket = -NOISE_ABSOLUTE_ZERO + 10.0 * SignalBwLog[_bandwidth] + NOISE_FIGURE + ( double )_SNR;

			// added by C. Pham, using Semtech SX1272 rev3 March 2015
			// for SX1272 we use -139, for SX1276, we use -157
			// then for SX1276 when using low-frequency (i.e. 433MHz) then we use -164
			//_RSSIpacket = -(OFFSET_RSSI+(_board==SX1276Chip?18:0)+(_channel<CH_04_868?7:0)) + (double)_RSSIpacket + (double)_rawSNR*0.25;
			if c.version == VersionSX1276 {
				if c.channel < CH_04_868 {
					rssi = -(OFFSET_RSSI + 25) + int16(rssiv) + int16(snr)/4
				} else {
					rssi = -(OFFSET_RSSI + 18) + int16(rssiv) + int16(snr)/4
				}
			} else {
				if c.channel < CH_04_868 {
					rssi = -(OFFSET_RSSI + 7) + int16(rssiv) + int16(snr)/4
				} else {
					rssi = -(OFFSET_RSSI + 18) + int16(rssiv) + int16(snr)/4
				}
			}
		} else {
			if c.version == VersionSX1276 {
				if c.channel < CH_04_868 {
					rssi = -(OFFSET_RSSI + 25) + int16(rssiv) + int16(snr)*16.0/15.0
				} else {
					rssi = -(OFFSET_RSSI + 18) + int16(rssiv) + int16(snr)*16.0/15.0
				}
			} else {
				if c.channel < CH_04_868 {
					rssi = -(OFFSET_RSSI + 7) + int16(rssiv) + int16(snr)*16.0/15.0
				} else {
					rssi = -(OFFSET_RSSI + 18) + int16(rssiv) + int16(snr)*16.0/15.0
				}
			}
		}

		c.Log(LogLevelVerbose, "RSSI packet value is %d.", rssi)
	} else {
		// RSSI packet doesn't exist in FSK mode
		c.Log(LogLevelWarning, "RSSI packet does not exist in FSK mode.")
	}
	return
}

func (c *Chip) setCR(cod byte) (err error) {

	c.Log(LogLevelDebug, "Starting 'setCR'.")

	st0, _ := c.readRegister(REG_OP_MODE) // Save the previous status

	if c.mode == ModemFSK {
		c.Log(LogLevelWarning, "Notice that FSK hasn't Coding Rate parameter, so you are configuring it in LoRa mode.")
		if err := c.SetLORA(); err != nil {
			return err
		}
	}

	c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE) // Set Standby mode to write in registers

	config1, _ := c.readRegister(REG_MODEM_CONFIG1) // Save config1 to modify only the CR

	// added by C. Pham
	if c.version == VersionSX1272 {
		switch cod {
		case CR_5:
			config1 = config1 & 207 // 0B11001111;	// clears bits 5 & 4 from REG_MODEM_CONFIG1
			config1 = config1 | 8   // 0B00001000;	// sets bit 3 from REG_MODEM_CONFIG1
			break
		case CR_6:
			config1 = config1 & 215 // 0B11010111;	// clears bits 5 & 3 from REG_MODEM_CONFIG1
			config1 = config1 | 16  // 0B00010000;	// sets bit 4 from REG_MODEM_CONFIG1
			break
		case CR_7:
			config1 = config1 & 223 // 0B11011111;	// clears bit 5 from REG_MODEM_CONFIG1
			config1 = config1 | 24  // 0B00011000;	// sets bits 4 & 3 from REG_MODEM_CONFIG1
			break
		case CR_8:
			config1 = config1 & 231 // 0B11100111;	// clears bits 4 & 3 from REG_MODEM_CONFIG1
			config1 = config1 | 32  // 0B00100000;	// sets bit 5 from REG_MODEM_CONFIG1
			break
		}
	} else {
		// SX1276
		config1 = config1 & 241 // 0B11110001;	// clears bits 3 - 1 from REG_MODEM_CONFIG1
		switch cod {
		case CR_5:
			config1 = config1 | 2 // 0B00000010;
			break
		case CR_6:
			config1 = config1 | 4 // 0B00000100;
			break
		case CR_7:
			config1 = config1 | 6 // 0B00000110;
			break
		case CR_8:
			config1 = config1 | 8 // 0B00001000;
			break
		}
	}

	c.writeRegister(REG_MODEM_CONFIG1, config1) // Update config1

	delay(100)

	config1, _ = c.readRegister(REG_MODEM_CONFIG1)

	// added by C. Pham
	var nshift uint = 3

	// only 1 right shift for SX1276
	if c.version == VersionSX1276 {
		nshift = 1
	}

	// ((config1 >> 3) & 0B0000111) ---> take out bits 5-3 from REG_MODEM_CONFIG1 (=_codingRate)
	switch cod {
	case CR_5:
		if ((config1 >> nshift) & 7) != 0x01 { // 0B0000111
			err = fmt.Errorf("can not set cod")
		}
		break
	case CR_6:
		if ((config1 >> nshift) & 7) != 0x02 {
			err = fmt.Errorf("can not set cod")
		}
		break
	case CR_7:
		if ((config1 >> nshift) & 7) != 0x03 {
			err = fmt.Errorf("can not set cod")
		}
		break
	case CR_8:
		if ((config1 >> nshift) & 7) != 0x04 {
			err = fmt.Errorf("can not set cod")
		}
		break
	}

	if c.isCR(cod) {
		c.codingRate = cod
		c.Log(LogLevelVerbose, "Coding Rate 0x%x has been successfully set.", cod)
	} else {
		err = fmt.Errorf("can not set cod")
		c.Log(LogLevelError, "There has been an error while configuring Coding Rate parameter.")
	}
	c.writeRegister(REG_OP_MODE, st0) // Getting back to previous status
	delay(100)
	return
}

func (c *Chip) setSF(spr uint32) (err error) {

	c.Log(LogLevelDebug, "Starting 'setSF'.")

	st0, _ := c.readRegister(REG_OP_MODE) // Save the previous status

	if c.mode == ModemFSK {
		c.Log(LogLevelWarning, "Notice that FSK hasn't Spreading Factor parameter, so you are configuring it in LoRa mode.")
		if err := c.SetLORA(); err != nil {
			return err
		}
	}

	// modified by C. Pham
	c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE) // LoRa standby mode
	var config1, config2, config3 byte
	config1, _ = c.readRegister(REG_MODEM_CONFIG1) // Save config1 to modify only the LowDataRateOptimize
	config2, _ = c.readRegister(REG_MODEM_CONFIG2) // Save config2 to modify SF value (bits 7-4)

	isLowDROp := false

	//Mandatory with SF_11/12 and BW_125 as symbol duration > 16ms)
	if (spr == SF_11 || spr == SF_12) && c.bandwidth == BW_125 {
		isLowDROp = true
	}

	//Mandatory with SF_12 and BW_250 as symbol duration > 16ms)
	if spr == SF_12 && c.bandwidth == BW_250 {
		isLowDROp = true
	}

	switch spr {
	case SF_6:
		config2 = config2 & 111 // 0B01101111;	// clears bits 7 & 4 from REG_MODEM_CONFIG2
		config2 = config2 | 96  // 0B01100000;	// sets bits 6 & 5 from REG_MODEM_CONFIG2
		break
	case SF_7:
		config2 = config2 & 127 // 0B01111111;	// clears bits 7 from REG_MODEM_CONFIG2
		config2 = config2 | 112 // 0B01110000;	// sets bits 6, 5 & 4
		break
	case SF_8:
		config2 = config2 & 143 // 0B10001111;	// clears bits 6, 5 & 4 from REG_MODEM_CONFIG2
		config2 = config2 | 128 // 0B10000000;	// sets bit 7 from REG_MODEM_CONFIG2
		break
	case SF_9:
		config2 = config2 & 159 // 0B10011111;	// clears bits 6, 5 & 4 from REG_MODEM_CONFIG2
		config2 = config2 | 144 // 0B10010000;	// sets bits 7 & 4 from REG_MODEM_CONFIG2
		break
	case SF_10:
		config2 = config2 & 175 // 0B10101111;	// clears bits 6 & 4 from REG_MODEM_CONFIG2
		config2 = config2 | 160 // 0B10100000;	// sets bits 7 & 5 from REG_MODEM_CONFIG2
		break
	case SF_11:
		config2 = config2 & 191 // 0B10111111;	// clears bit 6 from REG_MODEM_CONFIG2
		config2 = config2 | 176 // 0B10110000;	// sets bits 7, 5 & 4 from REG_MODEM_CONFIG2
		break
	case SF_12:
		config2 = config2 & 207 // 0B11001111;	// clears bits 5 & 4 from REG_MODEM_CONFIG2
		config2 = config2 | 192 // 0B11000000;	// sets bits 7 & 6 from REG_MODEM_CONFIG2
		break
	}

	// added by C. Pham
	if isLowDROp { // LowDataRateOptimize
		if c.version == VersionSX1272 {

			config1 = config1 | 1 // 0B00000001;
		} else {
			config3, _ := c.readRegister(REG_MODEM_CONFIG3)
			config3 = config3 | 8 // 0B00001000;
		}
	} else {
		// No LowDataRateOptimize
		if c.version == VersionSX1272 {
			config1 = config1 & 254 // 0B11111110;
		} else {
			config3, _ := c.readRegister(REG_MODEM_CONFIG3)
			config3 = config3 & 247 // 0B11110111;
		}
	}

	// added by C. Pham
	if c.version == VersionSX1272 {
		// set the AgcAutoOn in bit 2 of REG_MODEM_CONFIG2
		// modified by C. Pham
		config2 = config2 | 4 // 0B00000100;

		// Update config1 now for SX1272Chip
		c.writeRegister(REG_MODEM_CONFIG1, config1)
	} else {
		// set the AgcAutoOn in bit 2 of REG_MODEM_CONFIG3
		config3 = config3 | 4 // 0B00000100;

		// and update config3 now for SX1276Chip
		c.writeRegister(REG_MODEM_CONFIG3, config3)
	}

	// here we write the new SF
	c.writeRegister(REG_MODEM_CONFIG2, config2) // Update config2

	// Check if it is neccesary to set special settings for SF=6
	if spr == SF_6 {
		// Mandatory headerOFF with SF = 6 (Implicit mode)
		c.setHeaderOFF()

		// Set the bit field DetectionOptimize of
		// register RegLoRaDetectOptimize to value "0b101".
		c.writeRegister(REG_DETECT_OPTIMIZE, 0x05)

		// Write 0x0C in the register RegDetectionThreshold.
		c.writeRegister(REG_DETECTION_THRESHOLD, 0x0C)
	} else {
		// added by C. Pham
		c.setHeaderON()

		// LoRa detection Optimize: 0x03 --> SF7 to SF12
		c.writeRegister(REG_DETECT_OPTIMIZE, 0x03)

		// LoRa detection threshold: 0x0A --> SF7 to SF12
		c.writeRegister(REG_DETECTION_THRESHOLD, 0x0A)
	}

	c.writeRegister(REG_OP_MODE, st0) // Getting back to previous status
	delay(100)

	c.getSF() // read from registers

	if c.spreadingFactor == spr {
		// Checking available value for .spreadingFactor
		c.Log(LogLevelVerbose, "Spreading factor %d has been successfully set.", spr)
	} else {
		c.Log(LogLevelError, "There has been an error while setting the spreading factor.")
		err = fmt.Errorf("can not set spr")
	}
	return
}

func (c *Chip) setBW(bw byte) (err error) {
	c.Log(LogLevelDebug, "Starting 'setBW'.")

	state := 2

	if !c.isBW(bw) {
		c.Log(LogLevelWarning, "Bandwidth %x is not a correct value.", bw)
		return fmt.Errorf("invalidn band: %x", bw)
	}

	st0, _ := c.readRegister(REG_OP_MODE) // Save the previous status

	if c.mode == ModemFSK {
		c.Log(LogLevelWarning, "Notice that FSK hasn't Bandwidth parameter, so you are configuring it in LoRa mode.")
		if err := c.SetLORA(); err != nil {
			return err
		}
	}

	c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE) // LoRa standby mode
	config1, _ := c.readRegister(REG_MODEM_CONFIG1) // Save config1 to modify only the BW

	// added by C. Pham for SX1276
	if c.version == VersionSX1272 {
		switch bw {
		case BW_125:
			config1 = config1 & 62 // 0B00111110;	// clears bits 7 & 6 and 0 (no LowDataRateOptimize) from REG_MODEM_CONFIG1
			if c.spreadingFactor == 11 || c.spreadingFactor == 12 {
				// LowDataRateOptimize (Mandatory with BW_125 if SF_11/12)
				config1 = config1 | 1 // 0B00000001;
			}
			break
		case BW_250:
			config1 = config1 & 126 // 0B01111110;	// clears bit 7 and 0 (no LowDataRateOptimize) from REG_MODEM_CONFIG1
			config1 = config1 | 64  // 0B01000000;	// sets bit 6 from REG_MODEM_CONFIG1
			if c.spreadingFactor == 12 {
				// LowDataRateOptimize (Mandatory with BW_250 if SF_12)
				config1 = config1 | 1 // 0B00000001;
			}
			break
		case BW_500:
			config1 = config1 & 190 // 0B10111110;	//clears bit 6 and 0 (no LowDataRateOptimize) from REG_MODEM_CONFIG1
			config1 = config1 | 128 // 0B10000000;	//sets bit 7 from REG_MODEM_CONFIG1
			break
		}
	} else {
		// SX1276
		config1 = config1 & 15 // 0B00001111;	// clears bits 7 - 4 from REG_MODEM_CONFIG1
		config3, _ := c.readRegister(REG_MODEM_CONFIG3)
		config3 = config3 & 247 // 0B11110111; // clears bit 3 (no LowDataRateOptimize)

		switch bw {
		case BW_125:
			// 0111
			config1 = config1 | 112 // 0B01110000;
			if c.spreadingFactor == 11 || c.spreadingFactor == 12 {
				// LowDataRateOptimize (Mandatory with BW_125 if SF_11 or SF_12)
				config3 = config3 | 8 // 0B00001000;
			}
			break
		case BW_250:
			// 1000
			config1 = config1 | 128 // 0B10000000;
			if c.spreadingFactor == 12 {
				// LowDataRateOptimize (Mandatory with BW_250 if SF_12)
				config3 = config3 | 8 // 0B00001000;
			}
			break
		case BW_500:
			// 1001
			config1 = config1 | 144 // 0B10010000;
			break
		}

		c.writeRegister(REG_MODEM_CONFIG3, config3)
	}
	// end

	c.writeRegister(REG_MODEM_CONFIG1, config1) // Update config1

	delay(100)

	// now we check
	config1, _ = c.readRegister(REG_MODEM_CONFIG1)

	// added by C. Pham
	if c.version == VersionSX1272 {
		// (config1 >> 6) ---> take out bits 7-6 from REG_MODEM_CONFIG1 (=_bandwidth)
		switch bw {
		case BW_125:
			if (config1 >> 6) == SX1272_BW_125 {
				state = 0
				if c.spreadingFactor == 11 || c.spreadingFactor == 12 {
					if config1&Bit0 == 1 { // LowDataRateOptimize
						state = 0
					} else {
						state = 1
					}
				}
			}
			break
		case BW_250:
			if (config1 >> 6) == SX1272_BW_250 {
				state = 0
				if c.spreadingFactor == 12 {
					if config1&Bit0 == 1 {
						// LowDataRateOptimize
						state = 0
					} else {
						state = 1
					}
				}
			}
			break
		case BW_500:
			if (config1 >> 6) == SX1272_BW_500 {
				state = 0
			}
			break
		}
	} else {
		// (config1 >> 4) ---> take out bits 7-4 from REG_MODEM_CONFIG1 (=_bandwidth)

		config3, _ := c.readRegister(REG_MODEM_CONFIG3)

		switch bw {
		case BW_125:
			if (config1 >> 4) == BW_125 {
				state = 0

				if c.spreadingFactor == 11 || c.spreadingFactor == 12 {
					if config3&Bit3 == 1 {
						// LowDataRateOptimize
						state = 0
					} else {
						state = 1
					}
				}
			}
			break
		case BW_250:
			if (config1 >> 4) == BW_250 {
				state = 0

				if c.spreadingFactor == 12 {
					if config3&Bit3 == 1 {
						// LowDataRateOptimize
						state = 0
					} else {
						state = 1
					}
				}
			}
			break
		case BW_500:
			if (config1 >> 4) == BW_500 {
				state = 0
			}
			break
		}
	}

	c.getBW() // read from registers

	if c.bandwidth == bw {
		// Checking available value for .spreadingFactor
		c.Log(LogLevelVerbose, "Bandwidth %d has been successfully set.", bw)
	} else {
		c.Log(LogLevelError, "There has been an error while setting the bandwidth: state %d, write %d, read %d", state, bw, c.bandwidth)
		err = fmt.Errorf("can not set bw")
	}

	c.writeRegister(REG_OP_MODE, st0) // Getting back to previous status
	delay(100)
	return
}

func (c *Chip) SetPowerDBM(power byte) (err error) {

	if c.power == power {
		return
	}

	c.Log(LogLevelDebug, "Starting 'setPowerDBM'.")

	st0, _ := c.readRegister(REG_OP_MODE) // Save the previous status
	if c.mode == ModeLoRa {
		c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE)
	} else {
		c.writeRegister(REG_OP_MODE, FSK_STANDBY_MODE)
	}

	if power == 20 {
		return c.setPower('X')
	}

	if power > 14 {
		return fmt.Errorf("can not set power > 14")
	}

	// disable high power output in all other cases
	if c.version == VersionSX1272 {
		c.writeRegister(0x5A, 0x84)
	} else {
		c.writeRegister(0x4D, 0x84)
	}

	if power > 10 {
		// set RegOcp for OcpOn and OcpTrim
		// 130mA
		c.SetMaxCurrent(0x10)
	} else {
		// 100mA
		c.SetMaxCurrent(0x0B)
	}

	var value byte

	if c.version == VersionSX1272 {
		if c.needPABOOST {
			value = power - 2
			// we set the PA_BOOST pin
			value = value | 128 // 0B10000000;
		} else {
			value = power + 1
		}

		c.writeRegister(REG_PA_CONFIG, value)
	} else {
		// for the SX1276
		var pmax byte = 15

		// then Pout = Pmax-(15-_power[3:0]) if  PaSelect=0 (RFO pin for +14dBm)
		// so L=3dBm; H=7dBm; M=15dBm (but should be limited to 14dBm by RFO pin)

		// and Pout = 17-(15-_power[3:0]) if  PaSelect=1 (PA_BOOST pin for +14dBm)
		// so x= 14dBm (PA);
		// when p=='X' for 20dBm, value is 0x0F and RegPaDacReg=0x87 so 20dBm is enabled

		if c.needPABOOST {
			value = power - 17 + 15
			// we set the PA_BOOST pin
			value = value | 128 // 0B10000000;
		} else {
			value = power - pmax + 15
		}

		// set MaxPower to 7 -> Pmax=10.8+0.6*MaxPower [dBm] = 15
		value = value | 112 // 0B01110000;

		c.writeRegister(REG_PA_CONFIG, value)
	}

	c.power = power

	value, _ = c.readRegister(REG_PA_CONFIG)

	if value == c.power {
		c.Log(LogLevelVerbose, "Output power has been successfully set.")
	} else {
		c.Log(LogLevelError, "Can not set output power: expected 0x%x, got 0x%x", c.power, value)
		err = fmt.Errorf("can not set output power")
	}

	c.writeRegister(REG_OP_MODE, st0) // Getting back to previous status
	delay(100)
	return
}

func (c *Chip) setPower(p byte) (err error) {

	var RegPaDacReg byte = 0x5A
	if c.version == VersionSX1276 {
		RegPaDacReg = 0x4D
	}

	c.Log(LogLevelDebug, "Starting 'setPower'.")

	st0, _ := c.readRegister(REG_OP_MODE) // Save the previous status
	if c.mode == ModeLoRa {
		// LoRa Stdby mode to write in registers
		c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE)
	} else {
		// FSK Stdby mode to write in registers
		c.writeRegister(REG_OP_MODE, FSK_STANDBY_MODE)
	}

	var value byte

	switch p {
	// L = Low. On SX1272/76: PA0 on RFO setting
	// H = High. On SX1272/76: PA0 on RFO setting
	// M = MAX. On SX1272/76: PA0 on RFO setting

	// x = extreme; added by C. Pham. On SX1272/76: PA1&PA2 PA_BOOST setting
	// X = eXtreme; added by C. Pham. On SX1272/76: PA1&PA2 PA_BOOST setting + 20dBm settings

	// added by C. Pham
	//
	case 'x', 'X', 'M':
		value = 0x0F
		// SX1272/76: 14dBm
		break

		// modified by C. Pham, set to 0x03 instead of 0x00
	case 'L':
		value = 0x03
		// SX1272/76: 2dBm
		break

	case 'H':
		value = 0x07
		// SX1272/76: 6dBm
		break

	default:
		err = fmt.Errorf("invalid power value")
		break
	}

	// 100mA
	c.SetMaxCurrent(0x0B)

	if p == 'x' {
		// we set only the PA_BOOST pin
		// limit to 14dBm
		value = 0x0C
		value = value | 128 // 0B10000000
		// set RegOcp for OcpOn and OcpTrim
		// 130mA
		c.SetMaxCurrent(0x10)
	}

	if p == 'X' {
		// normally value = 0x0F;
		// we set the PA_BOOST pin
		value = value | 128 // 0B10000000
		// and then set the high output power config with register REG_PA_DAC
		c.writeRegister(RegPaDacReg, 0x87)
		// set RegOcp for OcpOn and OcpTrim
		// 150mA
		c.SetMaxCurrent(0x12)
	} else {
		// disable high power output in all other cases
		c.writeRegister(RegPaDacReg, 0x84)
	}

	// added by C. Pham
	if c.version == VersionSX1272 {
		// Pout = -1 + _power[3:0] on RFO
		// Pout = 2 + _power[3:0] on PA_BOOST
		// so: L=2dBm; H=6dBm, M=14dBm, x=14dBm (PA), X=20dBm(PA+PADAC)
		c.writeRegister(REG_PA_CONFIG, value) // Setting output power value
	} else {
		// for the SX1276

		// set MaxPower to 7 -> Pmax=10.8+0.6*MaxPower [dBm] = 15
		value = value | 112 // 0B01110000

		// then Pout = Pmax-(15-_power[3:0]) if  PaSelect=0 (RFO pin for +14dBm)
		// so L=3dBm; H=7dBm; M=15dBm (but should be limited to 14dBm by RFO pin)

		// and Pout = 17-(15-_power[3:0]) if  PaSelect=1 (PA_BOOST pin for +14dBm)
		// so x= 14dBm (PA);
		// when p=='X' for 20dBm, value is 0x0F and RegPaDacReg=0x87 so 20dBm is enabled

		c.writeRegister(REG_PA_CONFIG, value)
	}

	c.power = value

	value, _ = c.readRegister(REG_PA_CONFIG)

	if value == c.power {
		c.Log(LogLevelWarning, "Output power has been successfully set.")
	} else {
		c.Log(LogLevelError, "Can not set output power.")
		err = fmt.Errorf("can not set output power")
	}

	c.writeRegister(REG_OP_MODE, st0) // Getting back to previous status
	delay(100)
	return
}

func (c *Chip) setHeaderON() (err error) {
	c.Log(LogLevelDebug, "Starting 'setHeaderON'.")

	if c.mode == ModemFSK {
		c.Log(LogLevelWarning, "FSK mode packets hasn't header.")
		return fmt.Errorf("no packet header with FSK")
	}

	config1, _ := c.readRegister(REG_MODEM_CONFIG1) // Save config1 to modify only the header bit
	if c.spreadingFactor == 6 {
		c.Log(LogLevelWarning, "Mandatory implicit header mode with spreading factor = 6.")
		return nil
	}

	// added by C. Pham
	if c.version == VersionSX1272 {
		config1 = config1 & 251 // 0B11111011;		// clears bit 2 from config1 = headerON
	} else {
		config1 = config1 & 254 // 0B11111110;              // clears bit 0 from config1 = headerON
	}

	c.writeRegister(REG_MODEM_CONFIG1, config1) // Update config1

	if c.spreadingFactor != 6 {
		// checking headerON taking out bit 2 from REG_MODEM_CONFIG1
		config1, _ := c.readRegister(REG_MODEM_CONFIG1)
		// modified by C. Pham
		if (c.version == VersionSX1272 && config1&Bit2 == HEADER_ON) || (c.version == VersionSX1276 && config1&Bit0 == HEADER_ON) {
			c.header = true
			c.Log(LogLevelVerbose, "Header has been activated.")
		} else {
			c.Log(LogLevelError, "Can not activate header.")
			err = fmt.Errorf("can not activate header")
		}
	}
	return
}

func (c *Chip) setHeaderOFF() (err error) {

	c.Log(LogLevelDebug, "Starting 'setHeaderOFF'.")

	if c.mode == ModemFSK {
		c.Log(LogLevelWarning, "FSK mode packets hasn't header.")
		return fmt.Errorf("no packet header with FSK")
	}

	config1, _ := c.readRegister(REG_MODEM_CONFIG1) // Save config1 to modify only the header bit

	// modified by C. Pham
	if c.version == VersionSX1272 {
		config1 = config1 | 4 // 0B00000100;			// sets bit 2 from REG_MODEM_CONFIG1 = headerOFF
	} else {
		config1 = config1 | 1 // 0B00000001;                      // sets bit 0 from REG_MODEM_CONFIG1 = headerOFF
	}

	c.writeRegister(REG_MODEM_CONFIG1, config1) // Update config1

	config1, _ = c.readRegister(REG_MODEM_CONFIG1)

	if (c.version == VersionSX1272 && config1&Bit2 == HEADER_OFF) || (c.version == VersionSX1276 && config1&Bit0 == HEADER_OFF) {
		c.header = true
		c.Log(LogLevelVerbose, "Header has been deactivated.")
	} else {
		c.Log(LogLevelError, "Can not deactivate header.")
		err = fmt.Errorf("can not deactivate header")
	}
	return
}

func (c *Chip) getBW() (err error) {
	c.Log(LogLevelDebug, "Starting 'getBW'.")

	if c.mode == ModemFSK {
		c.Log(LogLevelWarning, "FSK mode hasn't bandwidth.")
		return fmt.Errorf("no bandwith with FSK")
	}
	var config1 byte

	if c.version == VersionSX1272 {
		// take out bits 7-6 from REG_MODEM_CONFIG1 indicates _bandwidth
		config1, _ = c.readRegister(REG_MODEM_CONFIG1)
		config1 = config1 >> 6
		// added by C. Pham
		// convert to common bandwidth values used by both SX1272 and SX1276
		config1 += 7
	} else {
		// take out bits 7-4 from REG_MODEM_CONFIG1 indicates _bandwidth
		config1, _ = c.readRegister(REG_MODEM_CONFIG1)
		config1 = config1 >> 4
	}

	c.bandwidth = config1

	if !c.isBW(c.bandwidth) {
		c.Log(LogLevelError, "There has been an error while getting bandwidth.")
		err = fmt.Errorf("can not get bandwith")
	}
	return
}

func (c *Chip) getSF() (err error) {

	if c.mode == ModemFSK {
		c.Log(LogLevelWarning, "FSK mode hasn't spreading factor")
		return fmt.Errorf("no spreading factor with FSK")
	}

	// take out bits 7-4 from REG_MODEM_CONFIG2 indicates _spreadingFactor
	config2, _ := c.readRegister(REG_MODEM_CONFIG2)
	config2 = config2 >> 4

	c.spreadingFactor = uint32(config2)

	if !c.isSF(uint32(config2)) {
		c.Log(LogLevelError, "There has been an error while getting spreading factor.")
		err = fmt.Errorf("can not get spreading factor")
	}
	return
}

func (c *Chip) isBW(band uint8) bool {
	// Checking available values for .bandwidth
	// added by C. Pham
	if c.version == VersionSX1272 {
		switch band {
		case BW_125, BW_250, BW_500:
			return true
		default:
			return false
		}
	}

	switch band {
	case BW_7_8, BW_10_4, BW_15_6, BW_20_8, BW_31_25, BW_41_7, BW_62_5, BW_125, BW_250, BW_500:
		return true
	default:
		return false
	}
}

func (c *Chip) isCR(cod uint8) bool {
	// Checking available values for .codingRate
	switch cod {
	case CR_5, CR_6, CR_7, CR_8:
		return true
	default:
		return false
	}
}

func (c *Chip) isSF(spr uint32) bool {
	// Checking available values for _spreadingFactor
	switch spr {
	case SF_6, SF_7, SF_8, SF_9, SF_10, SF_11, SF_12:
		return true
	default:
		return false
	}
}

var errOnlyLora = errors.New("modulation must be \"LORA\"")

func (c *Chip) Send(pkt *lora.TxPacket) (err error) {

	switch pkt.Modulation {
	case "LORA":
		if c.mode != ModeLoRa {
			if err = c.SetLORA(); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported mode: %q", pkt.Modulation)
	}

	channel := uint32(uint64(pkt.Freq<<19) / 32000000)

	if err := c.SetChannel(channel); err != nil {
		return err
	}

	if c.spreadingFactor != pkt.Datarate {
		if !c.isSF(pkt.Datarate) {
			return fmt.Errorf("invalid spreading factor: %v", pkt.Datarate)
		}
		if err = c.setSF(pkt.Datarate); err != nil {
			return err
		}
	}
	var bw = pkt.LoRaBW - 1
	if c.bandwidth != pkt.LoRaBW {
		if !c.isBW(bw) {
			return fmt.Errorf("invalid bandwith: %v", pkt.LoRaBW)
		}
		if err = c.setBW(bw); err != nil {
			return err
		}
	}
	var cr = pkt.LoRaCR - 4
	if c.codingRate != cr {
		if !c.isCR(cr) {
			return fmt.Errorf("invalid coding rate: %v", pkt.LoRaCR)
		}
		if err := c.setCR(cr); err != nil {
			return err
		}
	}

	return c.sendPacketTimeout(pkt.Data, 10000)
}

func (c *Chip) send(payload []byte) error {
	//c.setPacketType(PKT_TYPE_DATA | PKT_FLAG_DATA_DOWNLINK)
	return c.sendPacketTimeout(payload, 10000)
}

func (c *Chip) sendPacketTimeout(payload []byte, timeout uint16) (err error) {
	err = c.setPacket(payload)
	if err != nil {
		return
	}
	return c.sendWithTimeout(timeout)
}

func (c *Chip) setPacket(payload []byte) (err error) {

	c.Log(LogLevelDebug, "Starting 'setPacket'.")

	st0, _ := c.readRegister(REG_OP_MODE) // Save the previous status
	c.clearFlags()                        // Initializing flags

	if c.mode == ModeLoRa {
		c.writeRegister(REG_OP_MODE, LORA_STANDBY_MODE)
	} else {
		c.writeRegister(REG_OP_MODE, FSK_STANDBY_MODE)
	}

	err = c.setPacketLength(byte(len(payload)))
	c.writeRegister(REG_FIFO_ADDR_PTR, 0x80) // Setting address pointer in FIFO data buffer
	if err == nil {
		// Writing packet to send in FIFO
		// writeRegister(REG_FIFO, packet_sent.netkey[0]);
		// writeRegister(REG_FIFO, packet_sent.netkey[1]);
		//writeRegister(REG_FIFO, packet_sent.dst); 		// Writing the destination in FIFO
		// added by C. Pham
		//writeRegister(REG_FIFO, packet_sent.type); 		// Writing the packet type in FIFO
		//writeRegister(REG_FIFO, packet_sent.src);		// Writing the source in FIFO
		//writeRegister(REG_FIFO, packet_sent.packnum);	// Writing the packet number in FIFO
		// commented by C. Pham
		//writeRegister(REG_FIFO, packet_sent.length); 	// Writing the packet length in FIFO
		for _, b := range payload {
			c.writeRegister(REG_FIFO, b)
		}

		c.Log(LogLevelVerbose, "Send packet %v.", payload)
	}
	c.writeRegister(REG_OP_MODE, st0) // Getting back to previous status
	return
}

func (c *Chip) sendWithTimeout(wait uint16) (err error) {

	c.Log(LogLevelDebug, "Starting 'sendWithTimeout'.")

	var value byte

	var exitTime = time.Now().Add(time.Millisecond * time.Duration(wait))

	if c.mode == ModeLoRa { // LoRa mode
		c.clearFlags() // Initializing flags

		c.writeRegister(REG_OP_MODE, LORA_TX_MODE) // LORA mode - Tx

		value, _ = c.readRegister(REG_IRQ_FLAGS)
		// Wait until the packet is sent (TX Done flag) or the timeout expires
		//while ((bitRead(value, 3) == 0) && (millis() - previous < wait))
		for value&Bit3 == 0 && exitTime.After(time.Now()) {
			value, _ = c.readRegister(REG_IRQ_FLAGS)
			delay(100)
			// Condition to avoid an overflow (DO NOT REMOVE)
			//if( millis() < previous )
			//{
			//    previous = millis();
			//}
		}
	} else { // FSK mode
		c.writeRegister(REG_OP_MODE, FSK_TX_MODE) // FSK mode - Tx

		value, _ = c.readRegister(REG_IRQ_FLAGS2)
		// Wait until the packet is sent (Packet Sent flag) or the timeout expires
		//while ((bitRead(value, 3) == 0) && (millis() - previous < wait))
		for value&Bit3 == 0 && exitTime.After(time.Now()) {
			value, _ = c.readRegister(REG_IRQ_FLAGS2)
			delay(100)
			// Condition to avoid an overflow (DO NOT REMOVE)
			//if( millis() < previous )
			//{
			//    previous = millis();
			//}
		}
	}
	if value&Bit3 != 0 {
		c.Log(LogLevelVerbose, "Packet successfully sent.")
	} else {
		c.Log(LogLevelError, "Timeout has expired.")
		err = ErrTimeout
	}

	c.clearFlags()
	return
}
