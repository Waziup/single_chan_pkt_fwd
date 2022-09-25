package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	logger "log"
	"math"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/Waziup/single_chan_pkt_fwd/SX127X"
	"github.com/Waziup/single_chan_pkt_fwd/fwd"
	"github.com/Waziup/single_chan_pkt_fwd/lora"

	"periph.io/x/host/v3"
	_ "periph.io/x/periph/host/rpi"
)

var gwid uint64

var tx = make(chan *lora.TxPacket)
var stat = new(fwd.Statistic)

var servers []*net.UDPAddr

var laddr = &net.UDPAddr{
	Port: 0,
	IP:   net.ParseIP("0.0.0.0"),
}

var never = time.Duration(math.MaxInt64)

var checkReceived = time.Millisecond * 500

var tickerKeepalive = time.NewTicker(time.Second * 60)
var tickerStatusReport = time.NewTicker(time.Second * 240)

var socket *net.UDPConn

const LogLevelNone = 0
const LogLevelDebug = 5
const LogLevelVerbose = 4
const LogLevelNormal = 3
const LogLevelWarning = 2
const LogLevelError = 1

var logLevel int = LogLevelNormal

var logLevelStr = []string{
	"[     ] ",
	"[ERR  ] ",
	"[WARN ] ",
	"[     ] ",
	"[VERBO] ",
	"[DEBUG] ",
}

func fatal(format string, v ...interface{}) {
	logger.Fatalf("[FATAL] "+format, v...)
}

func log(level int, format string, v ...interface{}) {
	timestamp := time.Now().UTC().Format(time.RFC822)
	if level <= logLevel && level >= -1 && level < 6 {
		logger.Printf(logLevelStr[level]+ "||" + timestamp + "|| "+format, v...)
	}
}

func main() {
	host.Init()

	logger.SetFlags(0)

	ll := flag.String("l", "", "log level: error, warn, verbose, debug, none")
	flag.Parse()

	switch *ll {
	case "", "normal":
		// logLevel = LogLevelNormal
	case "error", "e":
		logLevel = LogLevelError
	case "warn", "w":
		logLevel = LogLevelWarning
	case "verbose", "v":
		logLevel = LogLevelVerbose
	case "debug", "d":
		logLevel = LogLevelDebug
	case "none", "n":
		logLevel = LogLevelNone
	default:
		fatal("unknown log level (-l): %q", *ll)
	}

	data, err := ioutil.ReadFile("global_conf.json")
	if err != nil {
		dir, _ := os.Getwd()
		fatal("open %s/global_conf.json: %v", dir, err)
	}

	var globalConfig GlobalConfig
	err = json.Unmarshal(data, &globalConfig)
	if err != nil {
		fatal("can not parse 'global_conf.json': %v", err)
	}

	if globalConfig.SX127XConf == nil {
		fatal("no SX127X_conf in config")
	}

	if globalConfig.GatewayConfig == nil {
		fatal("no gateway_conf in config")
	}

	if globalConfig.SX127XConf.LoRaBW == 0 {
		globalConfig.SX127XConf.LoRaBW = 125000 // BW 125
	}
	if globalConfig.SX127XConf.LoRaCR == "" {
		globalConfig.SX127XConf.LoRaCR = "4/5" //CR 4/5
	}
	if globalConfig.GatewayConfig.KeepaliveInterval != 0 {
		tickerKeepalive = time.NewTicker(time.Second * time.Duration(globalConfig.GatewayConfig.KeepaliveInterval))
		log(LogLevelVerbose, "using %d seconds gateway keepaliveInterval", globalConfig.GatewayConfig.KeepaliveInterval)
	}else{
		log(LogLevelVerbose, "using %d seconds gateway keepaliveInterval", 60)
		tickerKeepalive = time.NewTicker(time.Second * time.Duration(60))
	}
	if globalConfig.GatewayConfig.StatusReportInterval != 0 {
		log(LogLevelVerbose, "using %d seconds gateway StatusReportInterval", globalConfig.GatewayConfig.StatusReportInterval)
		tickerStatusReport = time.NewTicker(time.Second * time.Duration(globalConfig.GatewayConfig.StatusReportInterval))
	}else{
		log(LogLevelVerbose, "using %d seconds gateway StatusReportInterval", 240)
		tickerStatusReport = time.NewTicker(time.Second * time.Duration(240))
	}

	log(LogLevelVerbose, "using %d servers for upstream", len(globalConfig.GatewayConfig.Servers))

	servers = make([]*net.UDPAddr, 0, len(globalConfig.GatewayConfig.Servers))
	i := 0
	for _, server := range globalConfig.GatewayConfig.Servers {
		if server.Enabled {

			i++
			ip := net.ParseIP(server.Address)
			if ip == nil {
				addr, err := net.LookupIP(server.Address)
				if err != nil {
					log(LogLevelError, " server %d: %s:%d: %v", i, server.Address, server.PortUp, err)
					continue
				}
				ip = addr[0]
				log(LogLevelVerbose, " server %d: %s:%d (%s:%d)", i, server.Address, server.PortUp, ip, server.PortUp)
			} else {
				log(LogLevelVerbose, " server %d: %s:%d", i, server.Address, server.PortUp)
			}
			servers = append(servers, &net.UDPAddr{
				Port: server.PortUp,
				IP:   ip,
			})
		}
	}

	gwid, err = strconv.ParseUint(globalConfig.GatewayConfig.GatewayID, 16, 64)
	if err != nil {
		fatal("can not parse gateway_ID: %v", err)
	}

	log(LogLevelVerbose, "center frequency: %.2f Mhz", float64(globalConfig.SX127XConf.Freq)/1e6)
	log(LogLevelVerbose, "spreading factor: SF%d", globalConfig.SX127XConf.Datarate)

	log(LogLevelVerbose, "this is gateway id %X", gwid)

	socket, err = net.ListenUDP("udp", laddr)
	if err != nil {
		fatal("%v", err)
	}
	laddr = socket.LocalAddr().(*net.UDPAddr)
	log(LogLevelNormal, "listening on %s", laddr)

	upstream(&fwd.Packet{
		Ident: fwd.PullData,
		Token: fwd.RndToken(),
	})

	go downstream()
	run(globalConfig.SX127XConf, globalConfig.GatewayConfig)
}

var baseTime = time.Now()

func run(cfg *lora.Config, g_cfg *GatewayConfig) {
	radio, err := SX127X.Discover(cfg)
	if err != nil {
		fatal("can not activate radio: %v", err)
	}

	log(LogLevelNormal, "radio %s activated.", radio.Name())

	radio.Logger = logger.New(os.Stdout, "", 0)
	radio.LogLevel = logLevel

	var timeReceive = time.Now()
	time.Sleep(time.Millisecond * 500)

	// for true {
	// 	pkt := &lora.TxPacket{
	// 		Modulation: "LORA",
	// 		LoRaBW:     0x08,
	// 		Freq:       868100000,
	// 		LoRaCR:     0x05,
	// 		Datarate:   0x0c,
	// 		Power:      14, // max
	// 		Data:       []byte("Hello World :D"),
	// 	}
	// 	log(0, "tx: %s", pkt)
	// 	if err = radio.Send(pkt); err != nil { 
	// 		log(LogLevelError, "can not send packet: %v", err)
	// 	}
	// 	log(0, "tx: ok")
	// 	time.Sleep(time.Second * 40)
	// }

	doReceive := false
	timerSend := time.NewTimer(never)
	stat.Desc =  g_cfg.Description
	stat.Mail = g_cfg.Mail
	stat.Latitude = g_cfg.Latitude
	stat.Longitude = g_cfg.Longitude
	stat.Altitude = g_cfg.Altitude

	for true {

		if !doReceive {
			err := radio.Receive(cfg)
			if err != nil {
				fatal("can not receive: %v", err)
			}
			log(LogLevelNormal, "waiting for packets ...")
			doReceive = true
		}

		timerReceive := time.NewTimer(checkReceived)
		

		
		select {
			case pkt := <-chanTx:

				log(LogLevelNormal, "received packet from upstream")

				if pkt.Immediate {
					log(LogLevelNormal, "sending immediate packet ...")
					doReceive = false
					if err = radio.Send(pkt); err != nil {
						log(LogLevelError, "can not send packet: %v", err)
					}
					stat.Dwnb += 1
					continue
				}

				pkt.Power = 14
				doReceive = false

				timeSend := baseTime.Add(time.Duration(pkt.CountUs) * time.Microsecond)
				timeSend.Add(time.Second)
				diff := timeSend.Sub(time.Now())
				log(LogLevelNormal, "sending packet in %s, %s since last received", diff, timeSend.Sub(timeReceive))
				// diff -= 200 * time.Microsecond
				// time.Sleep(diff)
				// tools.Nanosleep(int32(diff / time.Nanosecond))
				log(LogLevelNormal, "tx: %s", pkt)
				if err = radio.Send(pkt); err != nil {
					log(LogLevelError, "can not send packet: %v", err)
				}
				log(LogLevelNormal, "tx: ok")

				// enqueue packet
				// if queue == nil {
				// 	queue = &Queue{pkt: pkt}
				// } else {
				// 	if queue.pkt.CountUs > pkt.CountUs {
				// 		queue = &Queue{
				// 			next: queue,
				// 			pkt:  pkt,
				// 		}
				// 	} else {
				// 		for q := queue; ; q = q.next {
				// 			if q.next == nil {
				// 				q.next = &Queue{pkt: pkt}
				// 				break
				// 			}
				// 			if q.next.pkt.CountUs > pkt.CountUs {
				// 				q.next = &Queue{
				// 					next: q.next,
				// 					pkt:  pkt,
				// 				}
				// 				break
				// 			}
				// 		}
				// 	}
				// }
				// queueSize++

				// diff = 0 * time.Millisecond
				// log(LogLevelNormal, "tx queue: %d packets, next packet in %s", queueSize, diff)
				// timerSend.Reset(diff)

			case <-timerReceive.C:
				pkts, err := radio.GetPacket()
				if err != nil {
					fatal("can not receive packets: %v", err)
				}
				timeReceive = time.Now()
				if pkts != nil {
					doReceive = false
					for _, pkt := range pkts {
						// pkt.StatCRC = 1
						pkt.CountUs = uint32(time.Now().Sub(baseTime) / time.Microsecond)
						log(LogLevelNormal, "rx: %s", pkt)
						stat.Rxnb +=1 
					}
					log(LogLevelNormal, "received %d packets, pushing to upstream ...", len(pkts))
					upstream(&fwd.Packet{
						Token:     fwd.RndToken(),
						Ident:     fwd.PushData,
						RxPackets: pkts,
					})
				}
				timerReceive.Reset(checkReceived)

			case <-timerSend.C:
				if queue == nil {
					break
				}
				pkt := queue.pkt
				queue = queue.next
				queueSize--

				log(LogLevelNormal, "tx: %s", pkt)

				doReceive = false
				if err = radio.Send(pkt); err != nil {
					log(LogLevelError, "tx: can not send packet: %v", err)
				}
				stat.Rxfw +=1
				log(LogLevelNormal, "tx: ok")

				if queue == nil {
					timerSend.Reset(never)
					log(LogLevelNormal, "tx queue: 0 packets (no pending packets)")
				} else {
					diff := baseTime.Add(time.Duration(queue.pkt.CountUs) * time.Microsecond).Sub(time.Now())
					log(LogLevelNormal, "tx queue: %d packets, next packet in %s", queueSize, diff)
					timerSend.Reset(diff)
				}

			case <-tickerKeepalive.C:

				upstream(&fwd.Packet{
					Ident: fwd.PullData,
					Token: fwd.RndToken(),
				})

			case <-tickerStatusReport.C:
				stat.TimeStamp = time.Now().UTC()
				fmt.Println("send statusReport", stat)
				upstream(&fwd.Packet{
						Token: fwd.RndToken(),
						Ident: fwd.PushData,
						Stat: stat,
					})
				stat.Rxnb = 0
				stat.Rxfw = 0
				stat.Dwnb = 0
		}
		
	}
}

func upstream(pkt *fwd.Packet) {
	pkt.GatewayID = gwid
	data, err := pkt.MarshalBinary()
	if err != nil {
		log(LogLevelError, "can not upstream packet: %v", err)
		log(LogLevelError, "packet: %+v", pkt)
		return
	}

	log(LogLevelDebug, "(-> *) raw: %q", data)

	if logLevel >= LogLevelDebug {
		pktJSON, err := json.Marshal(pkt)
		log(LogLevelDebug, "pkt json: %s (err:%v)", pktJSON, err)
	}

	for _, server := range servers {
		if _, err = socket.WriteToUDP(data, server); err != nil {
			log(LogLevelError, "(-> %s) can not write upstream: %v", server, err)
		} else {
			log(LogLevelNormal, "(-> %s) %s", server, pkt)
		}
	}
}

func downstream() {

	var buffer [2048]byte

	for true {
		l, raddr, err := socket.ReadFromUDP(buffer[:])
		if err != nil {
			fatal("%v", err)
		}

		log(LogLevelDebug, "(<- %s) raw: %q", raddr, buffer[:l])

		var pkt = &fwd.Packet{}
		err = pkt.UnmarshalBinary(buffer[:l])
		if err != nil {
			log(LogLevelError, "(<- %s) can not unmarshal downstream packet: %v", raddr, err)
			log(LogLevelNormal, "data: %q", buffer[:l])
			continue
		}

		log(LogLevelNormal, "(<- %s) %s", raddr, pkt)

		if pkt.TxPacket != nil {

			chanTx <- pkt.TxPacket

			upstream(&fwd.Packet{
				Token: pkt.Token,
				Ident: fwd.TxAck,
				TxAck: fwd.NoError,
			})
		}
	}
}

var chanTx = make(chan *lora.TxPacket)

type Queue struct {
	pkt  *lora.TxPacket
	next *Queue
}

var queue *Queue

var queueSize int
