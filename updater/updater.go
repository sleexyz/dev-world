// Bootstrap a development environment for a Go program
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"time"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "my string representation"
}
func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var serveFlag arrayFlags

type Updater struct {
	exitChan         chan struct{}
	shouldUpdateChan chan struct{}
	logger           *log.Logger
	cmds             map[*exec.Cmd]struct{}
	cmdMu            sync.Mutex
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	flag.Var(&serveFlag, "serve-flag", "flag to pass to serve")
	flag.Parse()

	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.SetPrefix("\033[33m[updater] \033[0m") // yellow

	u := &Updater{
		shouldUpdateChan: make(chan struct{}),
		exitChan:         make(chan struct{}),
		logger:           logger,
		cmds:             make(map[*exec.Cmd]struct{}),
		cmdMu:            sync.Mutex{},
	}

	u.freeUpPort(12345)
	u.freeUpPort(12344)

	go u.runClientDevServer()
	go u.runServe()
	go u.runUpdater()
	go u.runExtensionUpdater()
	go u.runPacmanExtensionUpdater()

	select {
	case <-c:
		break
	case <-u.exitChan:
		break
	}
	u.logger.Println("Shutting down...")
	u.cleanUp()
	os.Exit(0)
}

func (u *Updater) cleanUp() {
	for cmd := range u.cmds {
		log.Printf("Killing process %d", cmd.Process.Pid)
		cmd.Process.Kill()
	}
}

func (u *Updater) runClientDevServer() {
	u.loop(func() {
		cmd := exec.Command("sh", "-c", "cd client && npm run dev")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		u.logger.Println("Starting client dev server.")
		err := cmd.Start()
		if err != nil {
			u.logger.Panicf("Failed to start client dev server: %s", err)
		}
		u.cmdMu.Lock()
		u.cmds[cmd] = struct{}{}
		u.cmdMu.Unlock()

		defer func() {
			u.cmdMu.Lock()
			delete(u.cmds, cmd)
			u.cmdMu.Unlock()
		}()
		cmd.Wait()
	})
}

// Continuously run the program.
func (u *Updater) runServe() {
	u.loop(func() {
		cmd := exec.Command("bin/serve", serveFlag...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		u.logger.Println("Starting process.")
		err := cmd.Start()
		if err != nil {
			u.logger.Panicf("Failed to start process: %s", err)
		}
		u.cmdMu.Lock()
		u.cmds[cmd] = struct{}{}
		u.cmdMu.Unlock()
		defer func() {
			u.cmdMu.Lock()
			delete(u.cmds, cmd)
			u.cmdMu.Unlock()
		}()

		doneChan := make(chan error)
		go func() {
			doneChan <- cmd.Wait()
		}()
		select {
		case <-u.shouldUpdateChan:
			cmd.Process.Signal(os.Interrupt)
			u.logger.Println("Process killed by updater")
		case <-doneChan:
			u.logger.Println("Process exited")
		}
	})
}

func (u *Updater) runExtensionUpdater() {
	u.loop(func() {
		u.logger.Println("Running extension updater")
		cmd := exec.Command(
			"zsh",
			"-c",
			"-l",
			"cat <(git ls-files extension) <(git ls-files --others --exclude-standard extension) | entr -n -d -r -s 'task build-extension'",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Start()
		if err != nil {
			u.logger.Panicf("Failed to start process: %s", err)
		}
		u.cmdMu.Lock()
		u.cmds[cmd] = struct{}{}
		u.cmdMu.Unlock()
		defer func() {
			u.cmdMu.Lock()
			delete(u.cmds, cmd)
			u.cmdMu.Unlock()
		}()
		cmd.Wait()
	})
}

func (u *Updater) runPacmanExtensionUpdater() {
	u.loop(func() {
		u.logger.Println("Running PACman extension updater")
		cmd := exec.Command(
			"zsh",
			"-c",
			"-l",
			"(cat <(git ls-files pacman) <(git ls-files --others --exclude-standard pacman) | entr -n -d -r -s 'task build-pacman')",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Start()
		if err != nil {
			u.logger.Panicf("Failed to start process: %s", err)
		}
		u.cmdMu.Lock()
		u.cmds[cmd] = struct{}{}
		u.cmdMu.Unlock()
		defer func() {
			u.cmdMu.Lock()
			delete(u.cmds, cmd)
			u.cmdMu.Unlock()
		}()
		cmd.Wait()
	})
}

// Updater sends signals when the program rebuilds and should be restarted
func (u *Updater) runUpdater() {
	u.loop(func() {
		u.logger.Println("Running updater")
		cmd := exec.Command(
			"zsh",
			"-c",
			"-l",
			// NOTE: We need the -z flag to exit, which sends a binary signal to the updater process.
			"cat <(git ls-files) <(git ls-files --others --exclude-standard) | grep pkg | entr -n -d -p -r -s -c -z './build.sh'",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Start()
		if err != nil {
			u.logger.Panicf("Failed to start process: %s", err)
		}
		u.cmdMu.Lock()
		u.cmds[cmd] = struct{}{}
		u.cmdMu.Unlock()
		defer func() {
			u.cmdMu.Lock()
			delete(u.cmds, cmd)
			u.cmdMu.Unlock()
		}()
		cmd.Wait()
		// Don't kill the process if build command exited with a non-zero exit code
		if err != nil {
			u.logger.Printf("Failed to build`: %s\n", err)
			return
		}
		u.shouldUpdateChan <- struct{}{}
	})
}

// If a process is running on the given port, kill it.
func (u *Updater) freeUpPort(port int) {
	cmd := exec.Command("sh", "-c", "lsof -sTCP:LISTEN -i:"+fmt.Sprintf("%d", port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		u.logger.Printf("Port %d is already in use", port)
		cmd := exec.Command("sh", "-c", "lsof -sTCP:LISTEN -i:"+fmt.Sprintf("%d", port)+" | awk 'NR==2{print $2}' | xargs kill")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			u.logger.Printf("Failed to kill process listening on port %d: %s", port, err)
			u.exitChan <- struct{}{}
		}
	}
}

func (u *Updater) loop(fn func()) {
	lastSampleTime := time.Now()
	runs := 0
	for {
		runs += 1
		fn()
		if runs > 3 {
			if time.Since(lastSampleTime) < 20*time.Second {
				u.logger.Printf("Process exited too quickly (%d runs in %s)", runs, time.Since(lastSampleTime))
				u.exitChan <- struct{}{}
				break
			} else {
				lastSampleTime = time.Now()
				runs = 0
			}
		}
	}
}
