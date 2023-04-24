// Bootstrap a development environment for a Go program
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

var logger = log.New(os.Stdout, "", log.LstdFlags)

type State struct {
	shouldUpdateChan chan struct{}
}

func main() {
	assertFreePort(12345)
	assertFreePort(12344)
	logger.SetPrefix("\033[33m[updater] \033[0m") // yellow
	state := &State{
		shouldUpdateChan: make(chan struct{}),
	}
	go func() {
		runClientDevServer(state)
	}()
	go func() {
		runProgram(state)
	}()
	go func() {
		runUpdater(state.shouldUpdateChan)
	}()
	<-make(chan struct{})
}

func runClientDevServer(state *State) {
	loop(func() {
		cmd := exec.Command("sh", "-c", "cd extension && npm run dev")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		logger.Println("Starting client dev server.")
		err := cmd.Start()
		if err != nil {
			logger.Panicf("Failed to start client dev server: %s", err)
		}
		cmd.Wait()
	})
}

// Continuously run the program.
func runProgram(state *State) {
	loop(func() {
		cmd := exec.Command("bin/serve")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		logger.Println("Starting process.")
		err := cmd.Start()
		if err != nil {
			logger.Panicf("Failed to start process: %s", err)
		}

		doneChan := make(chan error)
		go func() {
			doneChan <- cmd.Wait()
		}()
		select {
		case <-state.shouldUpdateChan:
			cmd.Process.Kill()
			logger.Println("Process killed by updater")
		case <-doneChan:
			logger.Println("Process exited")
		}
	})
}

// Updater sends signals when the program rebuilds and should be restarted
func runUpdater(shouldUpdateChan chan struct{}) {
	loop(func() {
		logger.Println("Running updater")
		cmd := exec.Command(
			"zsh",
			"-c",
			"-l",
			"cat <(git ls-files) <(git ls-files --others --exclude-standard) | grep pkg | entr -n -d -p -r -s -z './build.sh'",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		// Don't kill the process if build command exited with a non-zero exit code
		if err != nil {
			logger.Printf("Failed to build`: %s\n", err)
			return
		}
		shouldUpdateChan <- struct{}{}
	})
}

func assertFreePort(port int) {
	cmd := exec.Command("sh", "-c", "lsof -sTCP:LISTEN -i:"+fmt.Sprintf("%d", port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		logger.Fatalf("Port %d is already in use", port)
	}
}

func loop(fn func()) {
	lastSampleTime := time.Now()
	runs := 0
	for {
		runs += 1
		fn()
		if runs > 3 {
			if time.Since(lastSampleTime) < 10*time.Second {
				logger.Fatalf("Process exited too quickly (%d runs in %s)", runs, time.Since(lastSampleTime))
			} else {
				lastSampleTime = time.Now()
				runs = 0
			}
		}
	}
}
