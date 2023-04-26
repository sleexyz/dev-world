package sitter

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/sleexyz/dev-world/pkg/workspace"
)

func InitializeSitter() *Sitter {
	sitter := CreateNewSitter()
	pattern := regexp.MustCompile(`code-server-(.+)\.sock`)
	files, err := os.ReadDir("/tmp")
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		info, err := file.Info()
		if err != nil || file.IsDir() || (info.Mode()&os.ModeSocket) == 0 {
			continue
		}
		if matches := pattern.FindStringSubmatch(file.Name()); len(matches) > 0 {
			key := matches[1]
			folder, err := base64.StdEncoding.DecodeString(key)
			if err != nil {
				log.Printf("Invalid workspace key: %s", key)
				continue
			}
			ws, err := sitter.reconnectToWorkspace(context.Background(), string(folder))
			if err != nil {
				ws = workspace.CreateWorkspace(context.Background(), string(folder))
			}
			sitter.addWorkspace(ws)
		}
	}
	return sitter
}

func (s *Sitter) reconnectToWorkspace(ctx context.Context, path string) (*workspace.Workspace, error) {
	key := base64.StdEncoding.EncodeToString([]byte(path))
	codeServerSocketPath := fmt.Sprintf("/tmp/code-server-%s.sock", key)

	_, err := os.Stat(codeServerSocketPath)
	if err != nil {
		return nil, err
	}

	process, err := getMatchingProcess(ctx, codeServerSocketPath)
	if err != nil {
		// If the socket exists but the process doesn't, remove the socket
		if err := os.Remove(codeServerSocketPath); err != nil {
			log.Fatalf("Failed to remove existing socket: %v", err)
		}
		log.Printf("Removed stale socket at %s\n", path)
		return nil, err
	}

	// If the socket already exists, try to reconnect to it
	workspace := &workspace.Workspace{
		Path:       path,
		SocketPath: codeServerSocketPath,
		Process:    process,
	}
	err = workspace.WaitForSocket(ctx)
	if err != nil {
		return nil, err
	}
	log.Printf("Reconnecting to existing socket for %s\n", path)
	return workspace, nil
}

func getMatchingProcess(ctx context.Context, socketPath string) (*os.Process, error) {
	cmd := exec.CommandContext(ctx, "pgrep", "-f", socketPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	outs := strings.Split(string(out), "\n")
	for _, out := range outs {
		pid, err := strconv.Atoi(string(out))
		if err != nil {
			continue
		}
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		return process, nil
	}
	return nil, fmt.Errorf("no matching process found")
}
