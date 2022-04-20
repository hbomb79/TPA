package main

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/hbomb79/TPA/api"
	"github.com/hbomb79/TPA/pkg"
	"github.com/hbomb79/TPA/processor"
	"github.com/hbomb79/TPA/ws"
)

var logger = pkg.Log.GetLogger("Main", pkg.ALL)

type Tpa struct {
	proc        *processor.Processor
	socketHub   *ws.SocketHub
	wsGateway   *api.WsGateway
	httpGateway *api.HttpGateway
	httpRouter  *api.Router
}

func NewTpa() *Tpa {
	proc, err := processor.NewProcessor()
	if err != nil {
		panic(err)
	}

	return &Tpa{
		proc:        proc,
		httpRouter:  api.NewRouter(),
		httpGateway: api.NewHttpGateway(proc),
		socketHub:   ws.NewSocketHub(),
		wsGateway:   api.NewWsGateway(proc),
	}
}

func (tpa *Tpa) newClientConnection() map[string]interface{} {
	return map[string]interface{}{
		"ffmpegOptions":   tpa.proc.KnownFfmpegOptions,
		"ffmpegMatchKeys": processor.FFMPEG_COMMAND_SUBSTITUTIONS,
	}
}

func (tpa *Tpa) Start() {
	// Start websocket, router and processor
	wg := &sync.WaitGroup{}

	logger.Emit(pkg.INFO, "Starting Processor\n")
	procReady := make(chan bool)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tpa.proc.Start(procReady); err != nil {
			logger.Emit(pkg.FATAL, "Failed to start Processor: %v", err.Error())
		}

		close(procReady)

		logger.Emit(pkg.STOP, "Processor shutdown, cleaning up supporting services...\n")
		tpa.socketHub.Close()
		tpa.httpRouter.Stop()
	}()

	// Wait for processor to be fully online before constructing other services that rely
	// on certain fields being set/populated.
	v, ok := <-procReady
	if v && ok {
		logger.Emit(pkg.INFO, "Confguring HTTP and Websocket routes...\n")
		tpa.setupRoutes()
		tpa.socketHub.WithConnectionCallback(tpa.newClientConnection)

		wg.Add(1)
		go func() {
			defer wg.Done()
			tpa.socketHub.Start()
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			tpa.httpRouter.Start(&api.RouterOptions{
				ApiPort: 8080,
				ApiHost: "0.0.0.0",
				ApiRoot: "/api/tpa",
			})
		}()
	}

	// Wait for all processes to finish
	wg.Wait()
}

// setupRoutes initialises the routes and commands for the HTTP
// REST router, and the websocket hub
func (tpa *Tpa) setupRoutes() {
	tpa.httpRouter.CreateRoute("v0/queue", "GET", tpa.httpGateway.HttpQueueIndex)
	tpa.httpRouter.CreateRoute("v0/queue/{id}", "GET", tpa.httpGateway.HttpQueueGet)
	tpa.httpRouter.CreateRoute("v0/queue/promote/{id}", "POST", tpa.httpGateway.HttpQueueUpdate)
	tpa.httpRouter.CreateRoute("v0/ws", "GET", tpa.socketHub.UpgradeToSocket)

	tpa.socketHub.BindCommand("QUEUE_INDEX", tpa.wsGateway.WsQueueIndex)
	tpa.socketHub.BindCommand("QUEUE_DETAILS", tpa.wsGateway.WsQueueDetails)
	tpa.socketHub.BindCommand("QUEUE_REORDER", tpa.wsGateway.WsQueueReorder)
	tpa.socketHub.BindCommand("TROUBLE_RESOLVE", tpa.wsGateway.WsTroubleResolve)
	tpa.socketHub.BindCommand("TROUBLE_DETAILS", tpa.wsGateway.WsTroubleDetails)
	tpa.socketHub.BindCommand("PROMOTE_ITEM", tpa.wsGateway.WsItemPromote)
	tpa.socketHub.BindCommand("PAUSE_ITEM", tpa.wsGateway.WsItemPause)
	tpa.socketHub.BindCommand("CANCEL_ITEM", tpa.wsGateway.WsItemCancel)

	tpa.socketHub.BindCommand("PROFILE_INDEX", tpa.wsGateway.WsProfileIndex)
	tpa.socketHub.BindCommand("PROFILE_CREATE", tpa.wsGateway.WsProfileCreate)
	tpa.socketHub.BindCommand("PROFILE_REMOVE", tpa.wsGateway.WsProfileRemove)
	tpa.socketHub.BindCommand("PROFILE_MOVE", tpa.wsGateway.WsProfileMove)
	tpa.socketHub.BindCommand("PROFILE_SET_MATCH_CONDITIONS", tpa.wsGateway.WsProfileSetMatchConditions)
	tpa.socketHub.BindCommand("PROFILE_TARGET_UPDATE_COMMAND", tpa.wsGateway.WsProfiletargetUpdateCommand)
	tpa.socketHub.BindCommand("PROFILE_TARGET_CREATE", tpa.wsGateway.WsProfileTargetCreate)
	tpa.socketHub.BindCommand("PROFILE_TARGET_REMOVE", tpa.wsGateway.WsProfileTargetRemove)
	tpa.socketHub.BindCommand("PROFILE_TARGET_MOVE", tpa.wsGateway.WsProfileTargetMove)
}

func (tpa *Tpa) OnProcessorUpdate(update *processor.ProcessorUpdate) {
	body := map[string]interface{}{"context": update}
	if update.UpdateType == processor.PROFILE_UPDATE {
		body["profiles"] = tpa.proc.Profiles.Profiles()
		body["targetOpts"] = tpa.proc.KnownFfmpegOptions
	}

	tpa.socketHub.Send(&ws.SocketMessage{
		Title: "UPDATE",
		Body:  body,
		Type:  ws.Update,
	})
}

// main() is the entry point to the program, from here will
// we load the users TPA configuration from their home directory,
// merging the configuration with the default config
func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Panicf(err.Error())
	}

	procCfg := new(processor.TPAConfig)
	if err := procCfg.LoadFromFile(filepath.Join(homeDir, ".config/tpa/config.yaml")); err != nil {
		panic(err)
	}

	tpa := NewTpa()
	tpa.proc.WithConfig(procCfg).WithNegotiator(tpa)

	tpa.Start()
}
