package main

import (
	"encoding/json"
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
)

var gwid uint64

var tx = make(chan *lora.TxPacket)

var servers []*net.UDPAddr

var laddr = &net.UDPAddr{
	Port: 0,
	IP:   net.ParseIP("0.0.0.0"),
}

var socket *net.UDPConn

const LogLevelNone = 0
const LogLevelDebug = 5
const LogLevelVerbose = 4
const LogLevelNormal = 3
const LogLevelWarning = 2
const LogLevelError = 1

var logLevel int = LogLevelVerbose

var logLevelStr = []string{
	"[     ] ",
	"[ERR  ] ",
	"[WARN ] ",
	"",
	"[VERBO] ",
	"[DEBUG] ",
}

func fatal(format string, v ...interface{}) {
	logger.Fatalf("[FATAL] "+format, v...)
}

func log(level int, format string, v ...interface{}) {
	if level <= logLevel && level >= -1 && level < 6 {
		logger.Printf(logLevelStr[level]+format, v...)
	}
}

func main() {

	logger.SetFlags(0)

	data, err := ioutil.ReadFile("global_conf.json")
	if err != nil {
		fatal("%v", err)
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

	log(LogLevelVerbose, "using %d servers for upstream", len(globalConfig.GatewayConfig.Servers))

	servers = make([]*net.UDPAddr, 0, len(globalConfig.GatewayConfig.Servers))
	i := 0
	for _, server := range globalConfig.GatewayConfig.Servers {
		if server.Enabled {
			log(LogLevelVerbose, " server %d: %s:%d", i, server.Address, server.PortUp)
			i++
			servers = append(servers, &net.UDPAddr{
				Port: server.PortUp,
				IP:   net.ParseIP(server.Address),
			})
		}
	}

	gwid, err := strconv.ParseUint(globalConfig.GatewayConfig.GatewayID, 16, 64)
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

	upstream(&fwd.Packet{
		Ident:     fwd.PullData,
		Token:     fwd.RndToken(),
		GatewayID: gwid,
	})

	go downstream()
	run(globalConfig.SX127XConf)
}

var baseTime = time.Now()

var never = time.Duration(math.MaxInt64)

var checkReceived = time.Millisecond * 500

var tickerKeepalive = time.NewTicker(time.Second * 10)

func run(cfg *lora.Config) {

	radio, err := SX127X.Discover()
	if err != nil {
		fatal("can not activate radio: %v", err)
	}

	log(LogLevelNormal, "radio %s activated.", radio.Name())

	radio.Logger = logger.New(os.Stdout, "[RADIO] ", 0)
	radio.LogLevel = logLevel

	err = radio.Init()
	if err != nil {
		fatal("can not init radio: %v", err)
	}

	doReceive := false

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
		timerSend := time.NewTimer(never)

		select {
		case pkt := <-chanTx:

			log(LogLevelNormal, "received from upstream: %+v", pkt)

			if pkt.Immediate {
				log(LogLevelNormal, "sending immediate packet ...")
				doReceive = false
				if err = radio.Send(pkt); err != nil {
					log(LogLevelError, "can not send packet: %v", err)
				}

				continue
			}

			// enqueue packet
			if queue == nil {
				queue = &Queue{pkt: pkt}
			} else {
				if queue.pkt.CountUs > pkt.CountUs {
					queue = &Queue{
						next: queue,
						pkt:  pkt,
					}
				} else {
					for q := queue; ; q = q.next {
						if q.next == nil {
							q.next = &Queue{pkt: pkt}
							break
						}
						if q.next.pkt.CountUs > pkt.CountUs {
							q.next = &Queue{
								next: q.next,
								pkt:  pkt,
							}
							break
						}
					}
				}
			}

		case <-timerReceive.C:
			pkts, err := radio.GetPacket()
			if err != nil {
				fatal("can not receive packets: %v", err)
			}
			if pkts != nil {
				doReceive = false
				for _, pkt := range pkts {
					log(LogLevelNormal, "> %#v", pkt)
				}
				log(LogLevelNormal, "received %d packets, pushing to upstream ...", len(pkts))
				upstream(&fwd.Packet{
					Token:     fwd.RndToken(),
					GatewayID: gwid,
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

			log(LogLevelNormal, "< %#v", pkt)

			doReceive = false
			if err = radio.Send(pkt); err != nil {
				log(LogLevelError, "can not send packet: %v", err)
			}

			if !timerSend.Stop() {
				<-timerSend.C // drain
			}

			if queue != nil {
				timerSend.Reset(baseTime.Add(time.Duration(queue.pkt.CountUs) * time.Microsecond).Sub(time.Now()))
			} else {
				timerSend.Reset(never)
			}

		case <-tickerKeepalive.C:

		}
	}
}

func upstream(pkt *fwd.Packet) {
	//log(LogLevelNormal, "upstream %+v", pkts)
	data, err := pkt.MarshalBinary()
	if err != nil {
		log(LogLevelError, "can not upstream packet: %v", err)
		return
	}

	for _, server := range servers {
		if _, err := socket.WriteToUDP(data, server); err != nil {
			log(LogLevelError, "(-> %s) can not write upstream: %v", server, err)
		} else {
			log(LogLevelNormal, "(-> %s) sent %s packet (token: %v)", server, pkt.Ident, pkt.Token)
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

		var pkt = fwd.Packet{}
		err = pkt.UnmarshalBinary(buffer[:l])
		if err != nil {
			log(LogLevelError, "(<- %s) can not unmarshal downstream packet: %v", raddr, err)
			log(LogLevelNormal, "data: %q", buffer[:l])
			continue
		}

		log(LogLevelNormal, "(<- %s) received %s packet (token: %v)", raddr, pkt.Ident, pkt.Token)

		if pkt.TxPacket != nil {

			chanTx <- pkt.TxPacket

			upstream(&fwd.Packet{
				Token:     pkt.Token,
				GatewayID: gwid,
				Ident:     fwd.TxAck,
				TxAck:     fwd.NoError,
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
