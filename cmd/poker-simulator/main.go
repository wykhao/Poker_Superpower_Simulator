package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"poker-human-simulator/internal/app"
	"poker-human-simulator/internal/storelock"
	"poker-human-simulator/internal/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		showStartupError(err)
		os.Exit(1)
	}
}
func run() error {
	var dir string
	var port int
	var noBrowser bool
	var versionOnly bool
	flag.StringVar(&dir, "data-dir", "", "History directory (default: user config directory/poker-human-simulator)")
	flag.IntVar(&port, "port", 8787, "Local port; 0 chooses an available port")
	flag.BoolVar(&noBrowser, "no-browser", false, "Do not open a browser automatically")
	flag.BoolVar(&versionOnly, "version", false, "Print version and exit")
	flag.Parse()
	if versionOnly {
		fmt.Println(app.Version)
		return nil
	}
	if port < 0 || port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535")
	}
	if dir == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		dir = filepath.Join(config, "poker-human-simulator")
	}
	lockFile, err := storelock.Acquire(dir)
	if err != nil {
		if !noBrowser {
			if address := existingAddress(dir); address != "" {
				return openBrowser(address)
			}
		}
		return err
	}
	defer lockFile.Close()
	a, err := app.Open(dir)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("cannot start local server (try --port 0): %w", err)
	}
	stop := make(chan struct{})
	var once sync.Once
	server := &http.Server{Handler: web.Handler(a, func() { once.Do(func() { close(stop) }) }), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	url := "http://" + listener.Addr().String()
	if err := os.WriteFile(filepath.Join(dir, "local-url"), []byte(url), 0600); err != nil {
		listener.Close()
		return err
	}
	fmt.Printf("Poker Superpower Simulator v%s\nOpen: %s\nHistory: %s\nClose with Ctrl+C or the Quit button.\n", app.Version, url, dir)
	errs := make(chan error, 1)
	go func() { errs <- server.Serve(listener) }()
	if !noBrowser {
		if err := openBrowser(url); err != nil {
			fmt.Fprintln(os.Stderr, "Open the URL above in your browser:", err)
		}
	}
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)
	select {
	case err := <-errs:
		if err != http.ErrServerClosed {
			return err
		}
	case <-stop:
	case <-interrupt:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func existingAddress(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "local-url"))
	if err != nil {
		return ""
	}
	u, err := url.Parse(strings.TrimSpace(string(data)))
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	client := http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	r, err := client.Get(strings.TrimRight(u.String(), "/") + "/api/state")
	if err != nil {
		return ""
	}
	defer r.Body.Close()
	if r.StatusCode == 200 && r.Header.Get("X-Poker-Simulator") == "human-local-v1" {
		return strings.TrimRight(u.String(), "/")
	}
	return ""
}
func openBrowser(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		c = exec.Command("open", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	if err := c.Start(); err != nil {
		return err
	}
	go c.Wait()
	return nil
}
