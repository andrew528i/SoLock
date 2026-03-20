package jsonrpc

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/solock/solock/internal/application"
)

type Server struct {
	app      *application.App
	sockPath string
	token    string
	listener net.Listener
	handler  *Handler
	shutdown chan struct{}
	wg       sync.WaitGroup
}

func NewServer(app *application.App, dataDir string) (*Server, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	sockPath := filepath.Join(dataDir, "solock.sock")

	if _, err := os.Stat(sockPath); err == nil {
		conn, dialErr := net.DialTimeout("unix", sockPath, time.Second)
		if dialErr == nil {
			conn.Close()
			return nil, fmt.Errorf("another instance is already running")
		}
		os.Remove(sockPath)
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	if err := os.Chmod(sockPath, 0600); err != nil {
		listener.Close()
		os.Remove(sockPath)
		return nil, fmt.Errorf("chmod: %w", err)
	}

	shutdown := make(chan struct{})
	handler := NewHandler(app, token, shutdown)

	return &Server{
		app:      app,
		sockPath: sockPath,
		token:    token,
		listener: listener,
		handler:  handler,
		shutdown: shutdown,
	}, nil
}

func (s *Server) Token() string {
	return s.token
}

func (s *Server) SocketPath() string {
	return s.sockPath
}

func (s *Server) Serve() error {
	go s.expiryWatcher()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return nil
			default:
				return fmt.Errorf("accept: %w", err)
			}
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) Stop() {
	select {
	case <-s.shutdown:
	default:
		close(s.shutdown)
	}
	s.listener.Close()
	s.wg.Wait()
	os.Remove(s.sockPath)
	s.app.Lock()
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-s.shutdown:
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			resp := errorResponse(nil, ErrCodeParse, "parse error")
			data, _ := json.Marshal(resp)
			conn.Write(append(data, '\n'))
			continue
		}

		if req.JSONRPC != "2.0" {
			resp := errorResponse(req.ID, ErrCodeInvalidReq, "invalid jsonrpc version")
			data, _ := json.Marshal(resp)
			conn.Write(append(data, '\n'))
			continue
		}

		var tokenParam struct {
			Token string `json:"token"`
		}
		if req.Params != nil {
			json.Unmarshal(req.Params, &tokenParam)
		}
		if tokenParam.Token != s.token {
			resp := errorResponse(req.ID, ErrCodeAuthFailed, "invalid token")
			data, _ := json.Marshal(resp)
			conn.Write(append(data, '\n'))
			continue
		}

		resp := s.handler.Handle(&req)
		data, _ := json.Marshal(resp)
		conn.Write(append(data, '\n'))
	}
}

func (s *Server) expiryWatcher() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdown:
			return
		case <-ticker.C:
			if s.app.IsExpired() {
				s.app.Lock()
			}
		}
	}
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
