// Bootstrap a development environment for a Go program
package main

import (
	"log"
	"os"
	"os/exec"
	"time"
)

var logger = log.New(os.Stdout, "", log.LstdFlags)

type State struct {
	shouldUpdateChan chan struct{}
	lastSampleTime   time.Time
	runs             int
}

func main() {
	logger.SetPrefix("\033[33m[updater] \033[0m") // yellow
	state := &State{
		shouldUpdateChan: make(chan struct{}),
		lastSampleTime:   time.Now(),
		runs:             0,
	}
	go func() {
		runProgram(state)
	}()
	go func() {
		runUpdater(state.shouldUpdateChan)
	}()
	<-make(chan struct{})
}

// Continuously run the program.
func runProgram(state *State) {
	for {
		cmd := exec.Command("bin/serve")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		logger.Println("Starting process.")
		err := cmd.Start()
		if err != nil {
			logger.Panicf("Failed to start process: %s", err)
		}
		state.runs += 1

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
		if state.runs > 3 {
			if time.Since(state.lastSampleTime) < time.Second {
				logger.Fatalf("Process exited too quickly (%d runs in %s)", state.runs, time.Since(state.lastSampleTime))
			} else {
				state.lastSampleTime = time.Now()
				state.runs = 0
			}
		}
	}
}

// Updater sends signals when the program rebuilds and should be restarted
func runUpdater(shouldUpdateChan chan struct{}) {
	for {
		logger.Println("Running updater")
		cmd := exec.Command("sh", "-c", "cat <(git ls-files) <(git ls-files --others --exclude-standard) | entr -n -d -p -r -s -z -c './build.sh'")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		// Don't kill the process if build command exited with a non-zero exit code
		if err != nil {
			logger.Printf("Failed to build`: %s\n", err)
			continue
		}
		shouldUpdateChan <- struct{}{}
	}
}
