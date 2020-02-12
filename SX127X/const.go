package SX127X

const (
	REG_FIFO                    = 0x00
	REG_OP_MODE                 = 0x01
	REG_BITRATE_MSB             = 0x02
	REG_BITRATE_LSB             = 0x03
	REG_FDEV_MSB                = 0x04
	REG_FDEV_LSB                = 0x05
	REG_FRF_MSB                 = 0x06
	REG_FRF_MID                 = 0x07
	REG_FRF_LSB                 = 0x08
	REG_PA_CONFIG               = 0x09
	REG_PA_RAMP                 = 0x0A
	REG_OCP                     = 0x0B
	REG_LNA                     = 0x0C
	REG_RX_CONFIG               = 0x0D
	REG_FIFO_ADDR_PTR           = 0x0D
	REG_RSSI_CONFIG             = 0x0E
	REG_FIFO_TX_BASE_ADDR       = 0x0E
	REG_RSSI_COLLISION          = 0x0F
	REG_FIFO_RX_BASE_ADDR       = 0x0F
	REG_RSSI_THRESH             = 0x10
	REG_FIFO_RX_CURRENT_ADDR    = 0x10
	REG_RSSI_VALUE_FSK          = 0x11
	REG_IRQ_FLAGS_MASK          = 0x11
	REG_RX_BW                   = 0x12
	REG_IRQ_FLAGS               = 0x12
	REG_AFC_BW                  = 0x13
	REG_RX_NB_BYTES             = 0x13
	REG_OOK_PEAK                = 0x14
	REG_RX_HEADER_CNT_VALUE_MSB = 0x14
	REG_OOK_FIX                 = 0x15
	REG_RX_HEADER_CNT_VALUE_LSB = 0x15
	REG_OOK_AVG                 = 0x16
	REG_RX_PACKET_CNT_VALUE_MSB = 0x16
	REG_RX_PACKET_CNT_VALUE_LSB = 0x17
	REG_MODEM_STAT              = 0x18
	REG_PKT_SNR_VALUE           = 0x19
	REG_AFC_FEI                 = 0x1A
	REG_PKT_RSSI_VALUE          = 0x1A
	REG_AFC_MSB                 = 0x1B
	REG_RSSI_VALUE_LORA         = 0x1B
	REG_AFC_LSB                 = 0x1C
	REG_HOP_CHANNEL             = 0x1C
	REG_FEI_MSB                 = 0x1D
	REG_MODEM_CONFIG1           = 0x1D
	REG_FEI_LSB                 = 0x1E
	REG_MODEM_CONFIG2           = 0x1E
	REG_PREAMBLE_DETECT         = 0x1F
	REG_SYMB_TIMEOUT_LSB        = 0x1F
	REG_RX_TIMEOUT1             = 0x20
	REG_PREAMBLE_MSB_LORA       = 0x20
	REG_RX_TIMEOUT2             = 0x21
	REG_PREAMBLE_LSB_LORA       = 0x21
	REG_RX_TIMEOUT3             = 0x22
	REG_PAYLOAD_LENGTH_LORA     = 0x22
	REG_RX_DELAY                = 0x23
	REG_MAX_PAYLOAD_LENGTH      = 0x23
	REG_OSC                     = 0x24
	REG_HOP_PERIOD              = 0x24
	REG_PREAMBLE_MSB_FSK        = 0x25
	REG_FIFO_RX_BYTE_ADDR       = 0x25
	REG_PREAMBLE_LSB_FSK        = 0x26
	// added by C. Pham
	REG_MODEM_CONFIG3 = 0x26
	// end
	REG_SYNC_CONFIG         = 0x27
	REG_SYNC_VALUE1         = 0x28
	REG_SYNC_VALUE2         = 0x29
	REG_SYNC_VALUE3         = 0x2A
	REG_SYNC_VALUE4         = 0x2B
	REG_SYNC_VALUE5         = 0x2C
	REG_SYNC_VALUE6         = 0x2D
	REG_SYNC_VALUE7         = 0x2E
	REG_SYNC_VALUE8         = 0x2F
	REG_PACKET_CONFIG1      = 0x30
	REG_PACKET_CONFIG2      = 0x31
	REG_DETECT_OPTIMIZE     = 0x31
	REG_PAYLOAD_LENGTH_FSK  = 0x32
	REG_NODE_ADRS           = 0x33
	REG_BROADCAST_ADRS      = 0x34
	REG_FIFO_THRESH         = 0x35
	REG_SEQ_CONFIG1         = 0x36
	REG_SEQ_CONFIG2         = 0x37
	REG_DETECTION_THRESHOLD = 0x37
	REG_TIMER_RESOL         = 0x38
	REG_TIMER1_COEF         = 0x39
	// added by C. Pham
	REG_SYNC_WORD = 0x39
	//end
	REG_TIMER2_COEF   = 0x3A
	REG_IMAGE_CAL     = 0x3B
	REG_TEMP          = 0x3C
	REG_LOW_BAT       = 0x3D
	REG_IRQ_FLAGS1    = 0x3E
	REG_IRQ_FLAGS2    = 0x3F
	REG_DIO_MAPPING1  = 0x40
	REG_DIO_MAPPING2  = 0x41
	REG_VERSION       = 0x42
	REG_AGC_REF       = 0x43
	REG_AGC_THRESH1   = 0x44
	REG_AGC_THRESH2   = 0x45
	REG_AGC_THRESH3   = 0x46
	REG_PLL_HOP       = 0x4B
	REG_TCXO          = 0x58
	REG_PA_DAC        = 0x5A
	REG_PLL           = 0x5C
	REG_PLL_LOW_PN    = 0x5E
	REG_FORMER_TEMP   = 0x6C
	REG_BIT_RATE_FRAC = 0x70
)

const (
	CH_04_868 = 0xD7CCCC // channel 04, central freq = 863.20MHz
	CH_05_868 = 0xD7E000 // channel 05, central freq = 863.50MHz
	CH_06_868 = 0xD7F333 // channel 06, central freq = 863.80MHz
	CH_07_868 = 0xD80666 // channel 07, central freq = 864.10MHz
	CH_08_868 = 0xD81999 // channel 08, central freq = 864.40MHz
	CH_09_868 = 0xD82CCC // channel 09, central freq = 864.70MHz
	//
	CH_10_868 = 0xD84CCC // channel 10, central freq = 865.20MHz
	CH_11_868 = 0xD86000 // channel 11, central freq = 865.50MHz
	CH_12_868 = 0xD87333 // channel 12, central freq = 865.80MHz
	CH_13_868 = 0xD88666 // channel 13, central freq = 866.10MHz
	CH_14_868 = 0xD89999 // channel 14, central freq = 866.40MHz
	CH_15_868 = 0xD8ACCC // channel 15, central freq = 866.70MHz
	CH_16_868 = 0xD8C000 // channel 16, central freq = 867.00MHz
	CH_17_868 = 0xD90000 // channel 17, central freq = 868.00MHz

	// added by C. Pham
	CH_18_868 = 0xD90666 // 868.1MHz for LoRaWAN test

	CH_00_900 = 0xE1C51E // channel 00, central freq = 903.08MHz
	CH_01_900 = 0xE24F5C // channel 01, central freq = 905.24MHz
	CH_02_900 = 0xE2D999 // channel 02, central freq = 907.40MHz
	CH_03_900 = 0xE363D7 // channel 03, central freq = 909.56MHz
	CH_04_900 = 0xE3EE14 // channel 04, central freq = 911.72MHz
	CH_05_900 = 0xE47851 // channel 05, central freq = 913.88MHz
	CH_06_900 = 0xE5028F // channel 06, central freq = 916.04MHz
	CH_07_900 = 0xE58CCC // channel 07, central freq = 918.20MHz
	CH_08_900 = 0xE6170A // channel 08, central freq = 920.36MHz
	CH_09_900 = 0xE6A147 // channel 09, central freq = 922.52MHz
	CH_10_900 = 0xE72B85 // channel 10, central freq = 924.68MHz
	CH_11_900 = 0xE7B5C2 // channel 11, central freq = 926.84MHz
	CH_12_900 = 0xE4C000 // default channel 915MHz, the module is configured with it

	// added by C. Pham
	CH_00_433 = 0x6C5333 // 433.3MHz
	CH_01_433 = 0x6C6666 // 433.6MHz
	CH_02_433 = 0x6C7999 // 433.9MHz
	CH_03_433 = 0x6C9333 // 434.3MHz
	// end
)

const (
	RF_IMAGECAL_AUTOIMAGECAL_MASK = 0x7F
	RF_IMAGECAL_AUTOIMAGECAL_ON   = 0x80
	RF_IMAGECAL_AUTOIMAGECAL_OFF  = 0x00 // Default

	RF_IMAGECAL_IMAGECAL_MASK  = 0xBF
	RF_IMAGECAL_IMAGECAL_START = 0x40

	RF_IMAGECAL_IMAGECAL_RUNNING = 0x20
	RF_IMAGECAL_IMAGECAL_DONE    = 0x00 // Default

	RF_IMAGECAL_TEMPCHANGE_HIGHER = 0x08
	RF_IMAGECAL_TEMPCHANGE_LOWER  = 0x00

	RF_IMAGECAL_TEMPTHRESHOLD_MASK = 0xF9
	RF_IMAGECAL_TEMPTHRESHOLD_05   = 0x00
	RF_IMAGECAL_TEMPTHRESHOLD_10   = 0x02 // Default
	RF_IMAGECAL_TEMPTHRESHOLD_15   = 0x04
	RF_IMAGECAL_TEMPTHRESHOLD_20   = 0x06

	RF_IMAGECAL_TEMPMONITOR_MASK = 0xFE
	RF_IMAGECAL_TEMPMONITOR_ON   = 0x00 // Default
	RF_IMAGECAL_TEMPMONITOR_OFF  = 0x01
)

//LORA MODES:
const (
	LORA_SLEEP_MODE   = 0x80
	LORA_STANDBY_MODE = 0x81
	LORA_TX_MODE      = 0x83
	LORA_RX_MODE      = 0x85
)

//FSK MODES:
const (
	FSK_SLEEP_MODE   = 0x00
	FSK_STANDBY_MODE = 0x01
	FSK_TX_MODE      = 0x03
	FSK_RX_MODE      = 0x05
)

const (
	HEADER_ON       = 0
	HEADER_OFF      = 1
	CRC_ON          = 1
	CRC_OFF         = 0
	LORA            = 1
	FSK             = 0
	BROADCAST_0     = 0x00
	MAX_LENGTH      = 255
	MAX_PAYLOAD     = 251
	MAX_LENGTH_FSK  = 64
	MAX_PAYLOAD_FSK = 60
	//modified by C. Pham, 7 instead of 5 because we added a type field which should be PKT_TYPE_ACK and the SNR
	ACK_LENGTH = 7
	// added by C. Pham

	OFFSET_RSSI           = 139
	NOISE_FIGURE          = 6.0
	NOISE_ABSOLUTE_ZERO   = 174.0
	MAX_TIMEOUT           = 10000 //10000 msec = 10.0 sec
	MAX_WAIT              = 12000 //12000 msec = 12.0 sec
	MAX_RETRIES           = 5
	CORRECT_PACKET        = 0
	INCORRECT_PACKET      = 1
	INCORRECT_PACKET_TYPE = 2
)

// LORA CODING RATE:
const (
	CR_5 = 0x01
	CR_6 = 0x02
	CR_7 = 0x03
	CR_8 = 0x04
)

// LORA SPREADING FACTOR:
const (
	SF_6  = 0x06
	SF_7  = 0x07
	SF_8  = 0x08
	SF_9  = 0x09
	SF_10 = 0x0A
	SF_11 = 0x0B
	SF_12 = 0x0C
)

//LORA BANDWIDTH:
// modified by C. Pham

const (
	SX1272_BW_125 = 0x00
	SX1272_BW_250 = 0x01
	SX1272_BW_500 = 0x02
)

// use the following constants with setBW()
const (
	BW_7_8   = 0x00
	BW_10_4  = 0x01
	BW_15_6  = 0x02
	BW_20_8  = 0x03
	BW_31_25 = 0x04
	BW_41_7  = 0x05
	BW_62_5  = 0x06
	BW_125   = 0x07
	BW_250   = 0x08
	BW_500   = 0x09
)

const (
	LNA_MAX_GAIN = 0x23
	LNA_OFF_GAIN = 0x00
	LNA_LOW_GAIN = 0x20
)

// just bits
const (
	Bit0 byte = 1 << iota
	Bit1
	Bit2
	Bit3
	Bit4
	Bit5
	Bit6
	Bit7
)

var modes = []struct {
	cr, sf, bw byte
}{
	{},
	{CR_5, SF_12, BW_125},
	{CR_5, SF_12, BW_250},
	{CR_5, SF_10, BW_125},
	{CR_5, SF_12, BW_500},
	{CR_5, SF_10, BW_250},
	{CR_5, SF_11, BW_250},
	{CR_5, SF_9, BW_250},
	{CR_5, SF_9, BW_500},
	{CR_5, SF_8, BW_500},
	{CR_5, SF_7, BW_500},
}

const (
	PKT_TYPE_MASK = 0xF0
	PKT_FLAG_MASK = 0x0F

	PKT_TYPE_DATA = 0x10
	PKT_TYPE_ACK  = 0x20

	PKT_FLAG_ACK_REQ        = 0x08
	PKT_FLAG_DATA_ENCRYPTED = 0x04
	PKT_FLAG_DATA_WAPPKEY   = 0x02
	PKT_FLAG_DATA_DOWNLINK  = 0x01
)
