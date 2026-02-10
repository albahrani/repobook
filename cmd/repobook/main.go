package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pkg/browser"

	"repobook/internal/server"
)

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: repobook <path> [--host HOST] [--port PORT] [--no-open]\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Starts a local Markdown viewer for a repository directory.\n")
		flag.PrintDefaults()
	}

	host := flag.String("host", "127.0.0.1", "Host/interface to bind to")
	port := flag.Int("port", 0, "Port to listen on (0 = auto)")
	noOpen := flag.Bool("no-open", false, "Do not open the browser automatically")
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}

	// Join all remaining arguments to handle paths with spaces
	// This allows: repobook C:\path with spaces\file.txt
	// as well as: repobook "C:\path with spaces\file.txt"
	pathArg := flag.Arg(0)
	if flag.NArg() > 1 {
		// Join all arguments with spaces
		args := make([]string, flag.NArg())
		for i := 0; i < flag.NArg(); i++ {
			args[i] = flag.Arg(i)
		}
		pathArg = filepath.Join(args...)
	}

	root, err := filepath.Abs(pathArg)
	if err != nil {
		fatal(err)
	}
	st, err := os.Stat(root)
	if err != nil {
		fatal(err)
	}
	if !st.IsDir() {
		fatal(errors.New("path must be a directory"))
	}

	s, err := server.New(server.Options{Root: root})
	if err != nil {
		fatal(err)
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fatal(err)
	}
	actualAddr := ln.Addr().String()
	url := fmt.Sprintf("http://%s/", actualAddr)

	httpServer := &http.Server{
		Handler:      s.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal(err)
		}
	}()

	fmt.Printf("repobook: serving %s\n", root)
	fmt.Printf("repobook: open %s\n", url)
	if assets := s.RepoAssetBaseURL(); assets != "" {
		fmt.Printf("repobook: repo assets %s\n", assets)
	}
	if !*noOpen {
		_ = browser.OpenURL(url)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	_ = s.Close()
}

func fatal(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "repobook: %v\n", err)
	os.Exit(1)
}
