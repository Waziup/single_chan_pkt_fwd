package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math"
	"net"
	"os"
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

func main() {

	data, err := ioutil.ReadFile("global_conf.json")
	if err != nil {
		log.Fatal(err)
	}

	var globalConfig GlobalConfig
	err = json.Unmarshal(data, &globalConfig)
	if err != nil {
		log.Fatalf("can not parse 'global_conf.json': %v", err)
	}

	if globalConfig.SX127XConf == nil {
		log.Fatalf("no SX127X_conf in config")
	}

	if globalConfig.GatewayConfig == nil {
		log.Fatalf("no gateway_conf in config")
	}

	log.Printf("using %d servers for upstream", len(globalConfig.GatewayConfig.Servers))

	servers = make([]*net.UDPAddr, 0, len(globalConfig.GatewayConfig.Servers))
	i := 0
	for _, server := range globalConfig.GatewayConfig.Servers {
		if server.Enabled {
			log.Printf(" server %d: %s:%d", i, server.Address, server.PortUp)
			i++
			servers = append(servers, &net.UDPAddr{
				Port: server.PortUp,
				IP:   net.ParseIP(server.Address),
			})
		}
	}

	// gwid, err := strconv.ParseInt(globalConfig.GatewayConfig.GatewayID, 16, 64)
	// if err != nil {
	// 	log.Fatalf("can not parse gateway_ID: %v", err)
	// }

	log.Printf("center frequency: %.2f Mhz", float64(globalConfig.SX127XConf.Freq)/1e6)
	log.Printf("spreading factor: SF%d", globalConfig.SX127XConf.Datarate)

	log.Printf("this is gateway id %X", gwid)

	socket, err = net.ListenUDP("udp", laddr)
	if err != nil {
		log.Fatal(err)
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
		log.Fatalf("can not activate radio: %v", err)
	}

	log.Printf("radio %s activated.", radio.Name())

	radio.Logger = log.New(os.Stdout, "radio: ", log.Flags())
	radio.LogLevel = SX127X.LogLevelVerbose

	err = radio.Init()
	if err != nil {
		log.Fatalf("can not init radio: %v", err)
	}

	doReceive := false

	for true {

		if !doReceive {
			err := radio.Receive(cfg)
			if err != nil {
				log.Fatalf("can not receive: %v", err)
			}
			log.Println("waiting for packets ...")
			doReceive = true
		}

		timerReceive := time.NewTimer(checkReceived)
		timerSend := time.NewTimer(never)

		select {
		case pkt := <-chanTx:

			log.Printf("received from upstream: %+v", pkt)

			if pkt.Immediate {
				log.Printf("sending immediate packet ...")
				doReceive = false
				if err = radio.Send(pkt); err != nil {
					log.Printf("can not send packet: %v", err)
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
				log.Fatalf("can not receive packets: %v", err)
			}
			if pkts != nil {
				doReceive = false
				for _, pkt := range pkts {
					log.Printf("> %#v", pkt)
				}
				log.Printf("received %d packets, pushing to upstream ...", len(pkts))
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

			log.Printf("< %#v", pkt)

			doReceive = false
			if err = radio.Send(pkt); err != nil {
				log.Printf("can not send packet: %v", err)
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
	//log.Printf("upstream %+v", pkts)
	data, err := pkt.MarshalBinary()
	if err != nil {
		log.Printf("can not upstream packet: %v", err)
		return
	}

	for _, server := range servers {
		if _, err := socket.WriteToUDP(data, server); err != nil {
			log.Printf("(-> %s) can not write upstream: %v", server, err)
		} else {
			log.Printf("(-> %s) sent %s packet (token: %v)", server, pkt.Ident, pkt.Token)
		}
	}
}

func downstream() {

	var buffer [2048]byte

	for true {
		l, raddr, err := socket.ReadFromUDP(buffer[:])
		if err != nil {
			log.Fatal(err)
		}

		var pkt = fwd.Packet{}
		err = pkt.UnmarshalBinary(buffer[:l])
		if err != nil {
			log.Printf("(<- %s) can not unmarshal downstream packet: %v", raddr, err)
			log.Printf("data: %q", buffer[:l])
			continue
		}

		log.Printf("(<- %s) received %s packet (token: %v)", raddr, pkt.Ident, pkt.Token)

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
