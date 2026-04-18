package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	drynn "github.com/mdhender/drynn"
	"github.com/mdhender/drynn/internal/config"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func main() {
	log.SetFlags(0)

	env := os.Getenv("DRYNN_ENV")
	if env == "" {
		env = "development"
	}
	if err := config.LoadDotfiles(env); err != nil {
		log.Fatalf("dotenv: %v\n", err)
	}

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "login":
		err = runLogin(os.Args[2:])
	case "logout":
		err = runLogout()
	case "health":
		err = runHealth(os.Args[2:])
	case "version":
		fmt.Println(drynn.Version().Core())
		return
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		log.Fatalf("unknown command %q", os.Args[1])
	}

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}
}

func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	email := fs.String("email", "", "account email address")
	password := fs.String("password", "", "account password")
	server := fs.String("server", "", "server URL (e.g. http://localhost:8080)")
	configPath := fs.String("config", "", "path to the server config file")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s login [flags]\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *email == "" || *password == "" {
		return fmt.Errorf("email and password are required")
	}

	session, err := loadSession()
	if err != nil {
		return err
	}

	if *configPath != "" && session.AccessToken != "" {
		return fmt.Errorf("existing session found; run `drynn logout` before using --config")
	}

	serverURL, err := resolveServerURL(*server, *configPath, session)
	if err != nil {
		return err
	}

	endpoint, err := url.JoinPath(serverURL, "/api/v1/login")
	if err != nil {
		return fmt.Errorf("build login URL: %w", err)
	}

	body, err := json.Marshal(map[string]string{
		"email":    *email,
		"password": *password,
	})
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("login failed: %s", apiErr.Error)
		}
		return fmt.Errorf("login failed: %s", resp.Status)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if err := saveSession(sessionData{
		ServerURL:    serverURL,
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}); err != nil {
		return err
	}

	fmt.Println("logged in")
	return nil
}

func runLogout() error {
	if err := clearTokens(); err != nil {
		return err
	}
	fmt.Println("logged out")
	return nil
}

func runHealth(args []string) error {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	server := fs.String("server", "", "server URL (e.g. http://localhost:8080)")
	configPath := fs.String("config", "", "path to the server config file")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s health [flags]\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	session, err := loadSession()
	if err != nil {
		return err
	}

	serverURL, err := resolveServerURL(*server, *configPath, session)
	if err != nil {
		return err
	}

	endpoint, err := url.JoinPath(serverURL, "/api/v1/health")
	if err != nil {
		return fmt.Errorf("build health URL: %w", err)
	}

	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return fmt.Errorf("health request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %s", resp.Status)
	}

	var result struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("status=%s version=%s\n", result.Status, result.Version)
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s <command> [flags]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  login     authenticate with the server")
	fmt.Fprintln(os.Stderr, "  logout    clear the current session")
	fmt.Fprintln(os.Stderr, "  health    check server health")
	fmt.Fprintln(os.Stderr, "  version   print the build version")
}
