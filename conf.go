package main

import "github.com/Waziup/single_chan_pkt_fwd/lora"

// GlobalConfig represents a "global_config.json" file.
type GlobalConfig struct {
	SX127XConf    *lora.Config   `json:"SX127X_conf"`
	GatewayConfig *GatewayConfig `json:"gateway_conf"`
}

// GatewayConfig ha sht egateway ID and lists servers that we connect to.
type GatewayConfig struct {
	GatewayID string `json:"gateway_ID"`
	Servers   []struct {
		Address  string `json:"server_address"`
		PortUp   int    `json:"serv_port_up"`
		PortDown int    `json:"serv_port_down"`
		Enabled  bool   `json:"serv_enabled"`
	} `json:"servers"`
}
