package main

import (
	"flag"
	"fmt"

	"redhat.com/milton/config"
	"redhat.com/milton/daemon"
)

func main() {
	writeUrl := flag.String("write-url", "", "Prometheus remote write endpoint")
	tick := flag.Uint("tick", 0, "Report every tick seconds")
	certPath := flag.String("cert", "", "Host certificate path")
	keyPath := flag.String("key", "", "Host certificate key path")
	configPath := flag.String("config", config.DefaultConfigPath, "Configuration file path")

	flag.NewFlagSet("help", flag.ExitOnError)
	flag.NewFlagSet("daemon", flag.ExitOnError)
	flag.NewFlagSet("once", flag.ExitOnError)
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Error: no subcommand specified")
		printUsage()
		return
	}

	command := args[0]
	switch command {
	case "help":
		printUsage()
	case "daemon", "once":
		cfg := config.NewConfig()
		cfg.UpdateFromConfigFile(*configPath)
		cfg.UpdateFromEnvVars()
		cfg.UpdateFromCliOptions(*writeUrl, *tick, *certPath, *keyPath)
		cfg.Print()
		d := daemon.NewDaemon(cfg)

		if command == "once" {
			d.RunOnce()
			return
		}
		d.Run()
	default:
		fmt.Println("Error: unknown subcommand", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: milton [OPTIONS] SUBCOMMAND")
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println("Subcommands:")
	fmt.Println("  daemon    Run in daemon mode")
	fmt.Println("  once      Execute once")
	fmt.Println("  help      Print this help message")
}
