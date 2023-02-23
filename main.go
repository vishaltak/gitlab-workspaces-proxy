package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/fsnotify/fsnotify"
	"gitlab.com/remote-development/auth-proxy/config"
	"gitlab.com/remote-development/auth-proxy/server"
)

func main() {

	port := flag.Int("port", 9876, "Port on which to listen")
	configFile := flag.String("config", "", "The config file to use")

	flag.Parse()

	configChannel := make(chan config.Config)

	ctx := context.Background()
	err := readConfigChange(ctx, *configFile, configChannel)

	opts := &server.ServerOptions{
		Port:          *port,
		ConfigChannel: configChannel,
	}

	s := server.New(opts)
	err = s.Start(ctx)
	if err != nil {
		fmt.Printf("Could not start server %s", err)
	}
}

func readConfigChange(ctx context.Context, filename string, notificationChannel chan<- config.Config) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Start listening for events.
	go func() {
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					log.Println("Config change detected")
					config, err := config.LoadConfig(filename)
					if err != nil {
						log.Printf("Error reading config file modification %s", err)
					} else {
						notificationChannel <- *config
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error in watching for config changes:", err)
			}
		}
	}()

	// Add a path.
	err = watcher.Add(filename)
	if err != nil {
		return err
	}

	config, err := config.LoadConfig(filename)
	if err != nil {
		return err
	}

	go func() {
		notificationChannel <- *config
	}()

	return nil
}