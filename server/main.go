package main

import (
	"bufio"
	"log"
	"net"
	"strconv"
	"sync"
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pb "phonecontrol/phonecontrol"
)

const (
	SERVE_PORT = "16868"
	TCP_PORT   = "18888"

	CHECK_CMD   = "checkstate"
	SILENCE_CMD = "setsilence"
	NORMAL_CMD  = "setnormal"
)

type server struct {
	hasClient  bool
	mutex      sync.Mutex

	state      chan int32
	setsilence chan int32
	setnormal  chan int32
	vibrate    chan int32
}

func (s *server) checkConnect() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.hasClient
}

func (s *server) setConnect(arg bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.hasClient = arg
}

func (s *server) GetVolumeState(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	log.Println("[GetVolumeState]: req:", req)
	if !s.checkConnect() {
		log.Println("[GetVolumeState]: no client")
		return &pb.Response{Result: -1}, nil
	}
	s.state <- 1
	result := <-s.state
	resq := pb.Response{Result: result}
	log.Println("[GetVolumeState]: resp:", resq)
	return &resq, nil
}

func (s *server) SetSilence(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	log.Println("[SetSilence]: req:", req)
	if !s.checkConnect() {
		log.Println("[SetSilence]: no client")
		return &pb.Response{Result: -1}, nil
	}
	s.setsilence <- 1
	result := <-s.setsilence
	resq := pb.Response{Result: result}
	log.Println("[SetSilence]: resp:", resq)
	return &resq, nil
}

func (s *server) SetNormal(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	log.Println("[SetNormal]: req:", req)
	if !s.checkConnect() {
		log.Println("[SetNormal]: no client")
		return &pb.Response{Result: -1}, nil
	}
	s.setnormal <- 1
	result := <-s.setnormal
	resq := pb.Response{Result: result}
	log.Println("[SetNormal]: resp:", resq)
	return &resq, nil
}

func (s *server) Vibrate(ctx context.Context, req *pb.VibrateRequest) (*pb.Response, error) {
	log.Println("[Vibrate]: req:", req)
	if !s.checkConnect() {
		log.Println("[Vibrate]: no client")
		return &pb.Response{Result: -1}, nil
	}
	s.vibrate <- int32(len(req.VTime))
	for _, value := range req.VTime {
		s.vibrate <- value
	}
	result := <-s.vibrate
	resq := pb.Response{Result: result}
	log.Println("[Vibrate]: resp:", resq)
	return &resq, nil
}

func handle(cmd string, conn net.Conn, ch chan int32) error {
	log.Printf("[handle]: send cmd '%s' to client\n", cmd)

	var result int32 = -1
	defer func() { ch <- result }()

	cmd += "\n"
	_, err := conn.Write([]byte(cmd))
	if err != nil {
		log.Println("[handle]: write error: ", err)
		return err
	}

	log.Println("[handle]: write successed")

	reader := bufio.NewReader(conn)
	data, err := reader.ReadString('\n')

	if err != nil {
		log.Println("[handle]: read from client failed: ", err)
		return err
	}

	log.Println("[handle]: read successed: ", data)
	n, err := strconv.ParseInt(strings.TrimSpace(data), 10, 32)
	if err != nil {
		log.Println("[handle]: convert error: ", err)
		return err
	}

	result = int32(n)
	return nil
}

func handleVibrate(num int32, conn net.Conn, ch chan int32) error {
	cmd := "vibrate\n"
	log.Println("[handleVibrate]: send cmd 'vibrate' to client")

	var result int32 = -1
	defer func() { ch <- result }()

	_, err := conn.Write([]byte(cmd))
	if err != nil {
		log.Println("[handleVibrate]: write error: ", err)
		return err
	}

	_, err = conn.Write([]byte(strconv.FormatInt(int64(num), 10) + "\n"))
	if err != nil {
		log.Println("[handleVibrate]: write error: ", err)
		return err
	}

	data := ""
	for i := int32(0); i < num; i++ {
		data += strconv.FormatInt(int64(<-ch), 10)
		if i != num-1 {
			data += ","
		}
	}
	data += "\n"
	_, err = conn.Write([]byte(data))
	if err != nil {
		log.Println("[handleVibrate]: write error: ", err)
		return err
	}

	log.Println("[handleVibrate]: write successed:%v", data)

	reader := bufio.NewReader(conn)
	ret, err := reader.ReadString('\n')
	if err != nil {
		log.Println("[handleVibrate]: read error: ", err)
		return err
	}

	log.Println("[handleVibrate]: read successed: ", result)
	n, err := strconv.ParseInt(strings.TrimSpace(ret), 10, 32)
	if err != nil {
		log.Println("[handleVibrate]: convert error: ", err)
		return err
	}

	result = int32(n)
	return nil
}

func tcpServerHandler(conn net.Conn, chs [4]chan int32, stop chan string) {
	defer conn.Close()
	var err error
	for {
		select {
		case <-chs[0]:
			err = handle(CHECK_CMD, conn, chs[0])
			if err != nil {
				preConn = nil
				sv.setConnect(false)
				return
			}
		case <-chs[1]:
			err = handle(SILENCE_CMD, conn, chs[1])
			if err != nil {
				preConn = nil
				sv.setConnect(false)
				return
			}
		case <-chs[2]:
			err = handle(NORMAL_CMD, conn, chs[2])
			if err != nil {
				preConn = nil
				sv.setConnect(false)
				return
			}
		case num := <-chs[3]:
			err = handleVibrate(num, conn, chs[3])
			if err != nil {
				preConn = nil
				sv.setConnect(false)
				return
			}
		case <-stop:
			sv.setConnect(false)
			stop<-conn.RemoteAddr().String()
			return
		}
	}
}

var preConn chan string
var sv = &server{state: make(chan int32),
	setnormal:  make(chan int32),
	setsilence: make(chan int32),
	vibrate:    make(chan int32),
	hasClient:  false,
}

func main() {
	tcpSoc, err := net.Listen("tcp", "0.0.0.0:"+TCP_PORT)
	if err != nil {
		log.Fatalf("Server failed to listen: %v", err)
	}

	log.Println("Server listen successed at tcp port:", TCP_PORT)
	go func() {
		for {
			conn, err := tcpSoc.Accept()
			if err != nil {
				log.Fatalf("Server accept error: %v", err)
				continue
			}

			log.Printf("Server accept successed [%v]\n", conn.RemoteAddr())

			if preConn != nil {
				preConn <- ""
				ret := <- preConn
				log.Printf("Close pre connection [%v]\n", ret)
			}else{
				preConn = make(chan string)
			}
			sv.setConnect(true)

			go tcpServerHandler(conn, [...]chan int32{sv.state, sv.setsilence, sv.setnormal, sv.vibrate},preConn)
		}
	}()

	list, err := net.Listen("tcp", "0.0.0.0:"+SERVE_PORT)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Println("Server listen successed at server port:", SERVE_PORT)
	s := grpc.NewServer()
	pb.RegisterServiceServer(s, sv)
	s.Serve(list)
}
