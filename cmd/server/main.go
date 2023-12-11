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
	s := socks5Proxy.NewProxy(conf.Server.WsHost, conf.Server.WsPort)
	go s.ListenAndServe()
	web.Start(conf.Server.WebHost, conf.Server.WebPort)
	ws.Start(conf.Server.WsServerHost, conf.Server.WsServerPort, conf.Server.WsAuthorization)

}
