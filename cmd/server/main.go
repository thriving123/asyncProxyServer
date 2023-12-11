package main

import (
	"asyncProxy/config"
	"asyncProxy/proxy/httpProxy"
	"asyncProxy/proxy/socks5Proxy"
	"asyncProxy/web"
	"asyncProxy/ws"
)

func main() {
	conf := config.NewConfig("./config.yml")
	p := httpProxy.NewProxy(conf.Server.HttpHost, conf.Server.HttpPort)
	go p.Listen()
	s := socks5Proxy.NewProxy(conf.Server.Socks5Host, conf.Server.Socks5Port)
	go s.ListenAndServe()
	go web.Start(conf.Server.WebHost, conf.Server.WebPort, conf.Server.WebUsername, conf.Server.WebPassword)
	ws.Start(conf.Server.WsServerHost, conf.Server.WsServerPort, conf.Server.WsServerAuthorization)
}
