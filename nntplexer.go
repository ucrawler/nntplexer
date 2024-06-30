package main

import (
	"flag"
	"github.com/google/uuid"
	"github.com/hashicorp/memberlist"
	"github.com/pires/go-proxyproto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/ini.v1"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"net"
	"net/http"
	"nntplexer/nntp/nntpserver"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"syscall"
	"time"
)

type Config struct {
	ServerConfig     `ini:"server"`
	DbConfig         `ini:"db"`
	MonitoringConfig `ini:"monitoring"`
	ClusterConfig    `ini:"cluster"`
}

type ServerConfig struct {
	Addr          string
	Port          int
	ProxyProtocol bool
}

type DbConfig struct {
	Dsn      string
	CacheTtl int
}

type MonitoringConfig struct {
	Addr     string
	Port     int
	Endpoint string
}

type ClusterConfig struct {
	Nodes         []string
	BindAddr      string
	BindPort      int
	AdvertiseAddr string
	AdvertisePort int
	SecretKey     string
}

var Version = "v0.0.3"

func main() {
	var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	var config = flag.String("config", "nntplexer.ini", "path to *.ini config file")

	flag.Parse()

	log.Printf("nntplexer %s is starting...\n", Version)
	go handleSignals()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	cfg := readConfig(*config)

	listener := initListener(&cfg.ServerConfig)
	defer listener.Close()

	db := initDb(&cfg.DbConfig)

	go initMonitoring(&cfg.MonitoringConfig)
	go initCluster(&cfg.ClusterConfig)

	ur := &UserRepository{db: db}
	br := &BackendRepository{db: db}
	ar := &ArticleRepository{db: db}
	pp := NewPoolProvider()

	schedule(func() {
		ur.Refresh()
		br.Refresh()
	}, 5*time.Second)

	schedule(func() {
		ar.Cleanup(cfg.DbConfig.CacheTtl)
	}, 1*time.Minute)

	server := nntpserver.NewServer(&NNTPBackend{
		ur: ur,
		br: br,
		ar: ar,
		pp: pp,
	})

	log.Println(server.Serve(listener))
}

func initListener(config *ServerConfig) net.Listener {
	addr := net.JoinHostPort(config.Addr, strconv.Itoa(config.Port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %v\n", listener.Addr())

	if config.ProxyProtocol {
		listener = &proxyproto.Listener{
			Listener: listener,
			Policy: func(upstream net.Addr) (proxyproto.Policy, error) {
				return proxyproto.REQUIRE, nil
			},
		}
	}

	return listener
}

func initMonitoring(monitoringConfig *MonitoringConfig) {
	addr := net.JoinHostPort(monitoringConfig.Addr, strconv.Itoa(monitoringConfig.Port))
	log.Printf("[monitoring] starting server on: %s\n", addr)

	http.Handle(monitoringConfig.Endpoint, promhttp.Handler())
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("[monitoring] init failed: %v\n", err)
	}
}

func initCluster(clusterConfig *ClusterConfig) {
	config := memberlist.DefaultWANConfig()
	config.BindAddr = clusterConfig.BindAddr
	config.BindPort = clusterConfig.BindPort
	config.AdvertiseAddr = clusterConfig.AdvertiseAddr
	config.AdvertisePort = clusterConfig.AdvertisePort
	config.SecretKey = []byte(clusterConfig.SecretKey)
	config.Delegate = &ClusterDelegate{}
	config.Name = uuid.NewString()

	mlist, err := memberlist.Create(config)
	if err != nil {
		log.Fatalf("[cluster] init failed: %v\n", err)
	}

	num, err := mlist.Join(clusterConfig.Nodes)
	if err != nil {
		log.Fatalf("[cluster] failed joining: %v\n", err)
	}

	log.Printf("[cluster] init done, nodes joined: %d", num)

	for _, node := range mlist.Members() {
		if mlist.LocalNode() == node {
			continue
		}

		err := mlist.SendReliable(node, []byte("hello"))
		if err != nil {
			log.Fatal(err)
		}
	}
}

func handleSignals() {
	sigchan := make(chan os.Signal, 1)

	signals := []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}
	for _, sig := range signals {
		if !signal.Ignored(sig) {
			signal.Notify(sigchan, sig)
		}
	}

	sig := <-sigchan

	log.Printf("Signal caught: %s\n", sig)
	log.Printf("Shutting down...\n")

	pprof.StopCPUProfile()

	os.Exit(1)
}

func schedule(what func(), delay time.Duration) {
	what()
	ticker := time.NewTicker(delay)
	go func() {
		for range ticker.C {
			what()
		}
	}()
}

func readConfig(path string) *Config {
	config := &Config{
		ServerConfig{Addr: "127.0.0.1", Port: 9999, ProxyProtocol: false},
		DbConfig{},
		MonitoringConfig{},
		ClusterConfig{},
	}

	err := ini.StrictMapToWithMapper(config, ini.TitleUnderscore, path)
	if err != nil {
		log.Fatal(err)
	}

	return config
}

func initDb(config *DbConfig) *gorm.DB {
	db, err := gorm.Open(mysql.Open(config.Dsn))
	if err != nil {
		log.Fatal(err)
	}

	err = db.AutoMigrate(&User{}, &Backend{}, &Article{})
	if err != nil {
		log.Fatal(err)
	}

	return db
}
