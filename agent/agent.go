package agent

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/sqreen/AgentGo/agent/backend"
	"github.com/sqreen/AgentGo/agent/backend/api"
	"github.com/sqreen/AgentGo/agent/config"
	"github.com/sqreen/AgentGo/agent/plog"
)

var token = os.Getenv("SQREEN_TOKEN")

func init() {
	start()
}

func start() {
	go agent()
}

func agent() {
	logger := plog.NewLogger("sqreen/agent")
	logger.SetLevel(plog.Info)
	logger.SetOutput(os.Stderr)

	client, err := backend.NewClient(config.BackendHTTPAPIBaseURL)
	if err != nil {
		logger.Fatal(err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		logger.Fatal(err)
	}

	binname, err := filepath.Abs(os.Args[0])
	if err != nil {
		logger.Fatal(err)
	}

	appLoginReq := api.AppLoginRequest{
		VariousInfos: api.AppLoginRequest_VariousInfos{
			Time: time.Now(),
			Pid:  uint32(os.Getpid()),
			Ppid: uint32(os.Getppid()),
			Euid: uint32(os.Geteuid()),
			Egid: uint32(os.Getegid()),
			Uid:  uint32(os.Getuid()),
			Gid:  uint32(os.Getgid()),
			Name: binname,
		},

		BundleSignature: "fixme",
		AgentType:       "golang",
		AgentVersion:    "0.0.0-0",
		OsType:          runtime.GOARCH + "-" + runtime.GOOS,
		Hostname:        hostname,
		RuntimeVersion:  runtime.Version(),
	}

	appLoginRes, err := client.AppLogin(&appLoginReq, token)
	if err != nil {
		logger.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Correctly stop the execution when receiving an interrupt signal
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		defer signal.Stop(c)
		<-c
		cancel()
		runtime.Gosched()
	}()

	heartbeat := time.Duration(appLoginRes.Features.HeartbeatDelay) * time.Second

	sessionKey := appLoginRes.SessionId

	logger.Info("Heartbeat!\n")

	var appBeatReq api.AppBeatRequest
	_, err = client.AppBeat(&appBeatReq, sessionKey)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Debug("up and running")

	for {
		select {
		// fixme: newtick > tick? to avoid leaks?
		case <-time.Tick(heartbeat):
			logger.Debug("heatbeat")
			var appBeatReq api.AppBeatRequest
			_, err := client.AppBeat(&appBeatReq, sessionKey)
			if err != nil {
				logger.Fatal(err)
			}

		case <-ctx.Done():
			err := client.AppLogout(sessionKey)
			if err != nil {
				logger.Fatal(err)
			}
			return
		}
	}
}
