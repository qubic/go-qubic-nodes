package main

import (
	"fmt"
	"github.com/ardanlabs/conf"
	"github.com/pkg/errors"
	"github.com/qubic/go-qubic-nodes/node"
	"github.com/qubic/go-qubic-nodes/web"
	"log"
	"net/http"
	"os"
	"time"
)

const prefix = "QUBIC_NODES"

type Configuration struct {
	Qubic struct {
		PeerList              []string      `conf:"default:5.39.222.64;82.197.173.130;82.197.173.129"`
		MaxPeers              int           `conf:"default:50"`
		ExchangeTimeout       time.Duration `conf:"default:2s"`
		MaxTickErrorThreshold uint32        `conf:"default:50"`
		ReliableTickRange     uint32        `conf:"default:30"`
	}
	Service struct {
		TickerUpdateInterval time.Duration `conf:"default:5s"`
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("main: exited with error: %s\n", err.Error())
	}
}

func run() error {

	var config Configuration

	if err := conf.Parse(os.Args[1:], prefix, &config); err != nil {
		switch err {
		case conf.ErrHelpWanted:
			usage, err := conf.Usage(prefix, &config)
			if err != nil {
				return errors.Wrap(err, "generating config usage")
			}
			fmt.Println(usage)
			return nil
		case conf.ErrVersionWanted:
			version, err := conf.VersionString(prefix, &config)
			if err != nil {
				return errors.Wrap(err, "generating config version")
			}
			fmt.Println(version)
			return nil
		}
		return errors.Wrap(err, "parsing config")
	}
	out, err := conf.String(&config)
	if err != nil {
		return errors.Wrap(err, "generating config for output")
	}
	log.Printf("main: Config :\n%v\n", out)

	container, err := node.NewNodeContainer(config.Qubic.PeerList, config.Qubic.MaxTickErrorThreshold, config.Qubic.ReliableTickRange, config.Qubic.ExchangeTimeout)
	if err != nil {
		log.Printf("Error: %v\n", err)
	}

	go func() {
		ticker := time.NewTicker(config.Service.TickerUpdateInterval)

		for {
			select {
			case <-ticker.C:
				updateErr := container.Update()
				if updateErr != nil {
					log.Printf("Error: %v\n", updateErr)
				}
			}
		}
	}()

	log.Printf("Staring WebServer...\n")

	handler := web.RequestHandler{
		Container: container,
	}

	http.HandleFunc("/status", handler.HandleStatus)
	http.HandleFunc("/max-tick", handler.HandleMaxTick)

	return http.ListenAndServe(":8080", nil)

}
