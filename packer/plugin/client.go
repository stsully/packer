package plugin

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"
)

type client struct {
	cmd *exec.Cmd
	exited bool
}

func NewClient(cmd *exec.Cmd) *client {
	return &client{
		cmd,
		false,
	}
}

func (c *client) Exited() bool {
	return c.exited
}

func (c *client) Start() (address string, err error) {
	env := []string{
		"PACKER_PLUGIN_MIN_PORT=10000",
		"PACKER_PLUGIN_MAX_PORT=25000",
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	c.cmd.Env = append(c.cmd.Env, env...)
	c.cmd.Stderr = stderr
	c.cmd.Stdout = stdout
	err = c.cmd.Start()
	if err != nil {
		return
	}

	// Make sure the command is properly cleaned up if there is an error
	defer func() {
		r := recover()

		if err != nil || r != nil {
			c.cmd.Process.Kill()
		}

		if r != nil {
			panic(r)
		}
	}()

	// Start goroutine to wait for process to exit
	go func() {
		c.cmd.Wait()
		c.exited = true
	}()

	// Start goroutine that logs the stderr
	go c.logStderr(stderr)

	// Some channels for the next step
	timeout := time.After(1 * time.Minute)

	// Start looking for the address
	for done := false; !done; {
		select {
		case <-timeout:
			err = errors.New("timeout while waiting for plugin to start")
			done = true
		default:
		}

		if err == nil && c.Exited() {
			err = errors.New("plugin exited before we could connect")
			done = true
		}

		if line, lerr := stdout.ReadBytes('\n'); lerr == nil {
			// Trim the address and reset the err since we were able
			// to read some sort of address.
			address = strings.TrimSpace(string(line))
			err = nil
			break
		}

		// If error is nil from previously, return now
		if err != nil {
			return
		}

		// Wait a bit
		time.Sleep(10 * time.Millisecond)
	}

	return
}

func (c *client) Kill() {
	c.cmd.Process.Kill()
}

func (c *client) logStderr(r io.Reader) {
	buf := bufio.NewReader(r)

	for done := false; !done; {
		if c.Exited() {
			done = true
		}

		var err error
		for err == nil {
			var line string
			line, err = buf.ReadString('\n')
			if line != "" {
				log.Print(line)
			}
		}

		time.Sleep(10 * time.Millisecond)
	}
}
