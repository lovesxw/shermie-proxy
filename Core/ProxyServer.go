package Core

import (
	"bufio"
	"fmt"
	"github.com/kxg3030/shermie-proxy/Core/Websocket"
	"github.com/kxg3030/shermie-proxy/Log"
	"github.com/viki-org/dnscache"
	"net"
	"net/http"
	"time"
)

var HttpHeadMap = map[string]int{
	// CONNECT
	"0x47": 0x47,
	// GET
	"0x43": 0x43,
	// POST|PUT
	"0x50": 0x50,
	// OPTIONS
	"0x4f": 0x4f,
	// DELETE
	"0x44": 0x44,
	// HEAD
	"0x48": 0x48,
}

type ProxyServer struct {
	port                  string
	listener              *net.TCPListener
	dns                   *dnscache.Resolver
	OnRequestEvent        func(request *http.Request)
	OnResponseEvent       func(response *http.Response)
	OnServerResponseEvent func(message []byte)
	OnClientSendEvent     func(message []byte)
	OnServerPacketEvent   func(msgType int, message []byte, clientConn *Websocket.Conn, resolve ResolveWs) error
	OnClientPacketEvent   func(msgType int, message []byte, tartgetConn *Websocket.Conn, resolve ResolveWs) error
}

func NewProxyServer(port string) *ProxyServer {
	p := &ProxyServer{
		port: port,
		dns:  dnscache.New(time.Minute * 5),
	}
	return p
}

func (i *ProxyServer) Start() error {
	// 解析地址
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%s", i.port))
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	// 监听服务
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	i.listener = listener
	Log.Log.Println("服务监听端口：" + i.port + "(如果是新生成的证书文件，请先手动将根证书.crt文件导入到系统中——by.失色天空)")
	i.MultiListen()
	select {}
}

func (i *ProxyServer) MultiListen() {
	for s := 0; s < 5; s++ {
		go func() {
			for {
				conn, err := i.listener.Accept()
				if err != nil {
					if e, ok := err.(net.Error); ok && e.Temporary() {
						Log.Log.Println("接受连接失败,重试：" + err.Error())
						time.Sleep(time.Second / 20)
					} else {
						Log.Log.Println("接受连接失败：" + err.Error())
					}
					continue
				}
				go i.handle(conn)
			}
		}()
	}
}

func (i *ProxyServer) handle(conn net.Conn) {
	defer conn.Close()
	// 使用bufio读取,原conn的句柄数据被读完
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	// 预读取一段字节,https、ws、wss读取到的数据为：CONNECT wan.xx.com:8080 HTTP/1.1
	peek, err := reader.Peek(1)
	if err != nil {
		return
	}
	peekHex := fmt.Sprintf("0x%x", peek[0])
	if peekHex == "0x5" {
		proxySocket := NewProxySocket()
		proxySocket.reader = reader
		proxySocket.writer = writer
		proxySocket.conn = conn
		proxySocket.server = i
		proxySocket.handle()
		return
	}
	if _, ok := HttpHeadMap[peekHex]; ok {
		proxyHttp := NewProxyHttp()
		proxyHttp.reader = reader
		proxyHttp.writer = writer
		proxyHttp.conn = conn
		proxyHttp.server = i
		proxyHttp.handle()
	}
}
