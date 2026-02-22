package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/chiisen/mini_bot/pkg/agent"
	"github.com/chiisen/mini_bot/pkg/config"
	"github.com/chiisen/mini_bot/pkg/logger"
)

// RunAgent handles the 'app agent' and 'app agent -m "..."' commands.
func RunAgent(args []string) error {
	logger.Init(false) // You could add a --debug flag handling here

	// Load Config
	cfg, err := config.Load("~/.minibot.go/config.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %w. Have you run 'onboard'?", err)
	}

	// Create Agent Instance
	instance, err := agent.NewInstance(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize agent instance: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Check if this is a single interaction or interactive
	isSingleMessage := false
	var singleMessageContent string

	for i, arg := range args {
		if arg == "-m" && i+1 < len(args) {
			isSingleMessage = true
			singleMessageContent = args[i+1]
			break
		}
	}

	sessionKey := "cli_default"

	// Define the print reply callback
	printReply := func(msg string) {
		fmt.Printf("Agent: %s\n", msg)
	}

	if isSingleMessage {
		// Single run mode
		return instance.Run(ctx, sessionKey, singleMessageContent, printReply)
	}

	// Interactive mode
	fmt.Println("🚀 MiniBot.go Interactive Mode Started (type 'exit' or 'quit' to leave)")
	
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fmt.Println("\nInterrupt received. Exiting...")
		cancel()
		os.Exit(0)
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}
		
		input := strings.TrimSpace(scanner.Text())
		if input == "exit" || input == "quit" {
			break
		}
		if input == "" {
			continue
		}

		if err := instance.Run(ctx, sessionKey, input, printReply); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

	return nil
}
