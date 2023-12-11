package config

import (
	"gopkg.in/yaml.v3"
	"log"
	"os"
)

type Config struct {
	// 服务端
	Server struct {
		// http服务端口
		HttpHost string `yaml:"http_host"`
		HttpPort uint16 `yaml:"http_port"`
		// ws代理服务端口
		Socks5Host string `yaml:"socks5_host"`
		Socks5Port uint16 `yaml:"socks5_port"`
		// websocket和client通讯端口
		WsServerHost          string `yaml:"ws_server_host"`
		WsServerPort          uint16 `yaml:"ws_server_port"`
		WsServerAuthorization string `yaml:"ws_server_authorization"`
		// web展示端口
		WebHost     string `yaml:"web_host"`
		WebPort     uint16 `yaml:"web_port"`
		WebUsername string `yaml:"web_username"`
		WebPassword string `yaml:"web_password"`
	} `yaml:"server"`
	Client struct {
		// 客户端连接的服务端地址
		ServerHost          string `yaml:"server_host"`
		ServerPort          uint16 `yaml:"server_port"`
		ServerAuthorization string `yaml:"server_authorization"`
		ServerSecure        bool   `yaml:"server_secure"`
	} `yaml:"client"`
}

func NewConfig(path string) *Config {
	// 读取config.yml
	file, err := os.ReadFile(path)
	// log.Println(string(file))
	if err != nil {
		log.Fatalln("读取配置文件出错:", err)
	}
	var config Config
	//用yaml解析
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		log.Fatalln("解析配置文件出错:", err)
	}
	return &config
}
