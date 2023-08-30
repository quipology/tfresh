/*
 * Filename: main.go
 * Author: Bobby Williams <bobwilliams@####.com>
 *
 * Copyright (c) 2023 ######
 *
 * Description: This tool refreshes/jumpstarts Palo Alto firewall VPN tunnels defined in a YAML configuration file.
 */

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	yaml "gopkg.in/yaml.v3"
)

const (
	// Palo commands to jumpstart VPN tunnels
	ikeSA   = "test vpn ike-sa gateway"
	ipsecSA = "test vpn ipsec-sa tunnel"

	// Palo Firewalls
	testFW = "palo-test-fw01.****.com"
	prodFW = "palo-prod-fw1.****.com"

	// Default SSH port
	sshPort = ":22"
)

var (
	// Configuration file to load
	configFile = "config.yml"

	// Iteration time
	iTime = 15 // 15 minutes
)

// 'customer' type represents a customer VPN connection
type customer struct {
	Name    string `yaml:"customer_name"`
	Gateway string `yaml:"customer_gateway"`
	Tunnel  string `yaml:"customer_tunnel"`
}

func main() {
	// Check for required environment variables
	username, password := checkEnvVars()

	// Process CLI flags
	flag.StringVar(&configFile, "c", configFile, fmt.Sprintf("Configuration filename (default is config.yml). Example: '%s -c custom.yml'", os.Args[0]))
	flag.IntVar(&iTime, "i", iTime, "Iteration interval (default 15 minutes)")
	fwEnv := flag.String("e", "", fmt.Sprintf("Firewall environment (prod, test). Example: '%s -e prod'", os.Args[0]))
	flag.Parse()

	// Set firewall environment
	var firewall string
	switch {
	case *fwEnv == "":
		fmt.Fprintln(os.Stderr, "[ERROR]: Firewall environment needs to be set.")
		flag.Usage()
		os.Exit(1)
	case *fwEnv == "prod":
		firewall = prodFW
	case *fwEnv == "test":
		firewall = testFW
	}

	// Load configuration file
	fBytes, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var customers []customer

	if err = yaml.Unmarshal(fBytes, &customers); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// SSH Connection Settings
	config := ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp4", firewall+sshPort, &config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	counter := 1
	for {
		fmt.Println("Starting iteration #", counter)

		session, err := client.NewSession()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer session.Close()

		pipe, err := session.StdinPipe()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer pipe.Close()

		if err = session.Shell(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Loop over customers from configuration file and jumpstart the tunnels
		for _, customer := range customers {
			fmt.Println("Refreshing connection:", customer.Name)
			runCMD(pipe, fmt.Sprintf("%s %s", ikeSA, customer.Gateway))
			runCMD(pipe, fmt.Sprintf("%s %s", ipsecSA, customer.Tunnel))
			fmt.Println("Refresh complete for:", customer.Name)
			fmt.Println(strings.Repeat("-", 30))
		}

		pipe.Close()
		session.Close()
		fmt.Printf("Processing Complete for iteration # %v.\n", counter)
		counter++
		fmt.Printf("Waiting for next iteration (%v)..\n", counter)
		time.Sleep(time.Duration(iTime) * time.Minute)
	}
}

// Check if environment variables are set
func checkEnvVars() (user, pass string) {
	username, exist := os.LookupEnv("PAN_USERNAME")
	if !exist {
		fmt.Fprintln(os.Stderr, "PAN_USERNAME environment variable not set.")
		os.Exit(1)
	} else {
		if username == "" {
			fmt.Fprintln(os.Stderr, "PAN_USERNAME cannot be blank.")
			os.Exit(1)
		} else {
			user = username
		}
	}

	password, exist := os.LookupEnv("PAN_PASSWORD")
	if !exist {
		fmt.Fprintln(os.Stderr, "PAN_PASSWORD environment variable not set.")
		os.Exit(1)
	} else {
		if username == "" {
			fmt.Fprintln(os.Stderr, "PAN_PASSWORD cannot be blank.")
			os.Exit(1)
		} else {
			pass = password
		}
	}
	return
}

// Utility function for executing shell commands
func runCMD(w io.Writer, cmd string) {
	fmt.Println("Executing:", cmd)
	fmt.Fprint(w, cmd+"\n")
	time.Sleep(2 * time.Second)
	fmt.Println("Execution Complete")
}
