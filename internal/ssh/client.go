package ssh

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/avast/retry-go/v4"
	"golang.org/x/crypto/ssh"
)

func WaitForSSH(
	ctx context.Context,
	addr string,
	sshUser string,
	sshPassword string,
) (*ssh.Client, error) {
	var sshConn ssh.Conn
	var chans <-chan ssh.NewChannel
	var reqs <-chan *ssh.Request

	if err := retry.Do(func() error {
		var netConn net.Conn
		var err error

		boundedCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		dialer := net.Dialer{}
		netConn, err = dialer.DialContext(boundedCtx, "tcp", addr)
		if err != nil {
			return err
		}

		sshConfig := &ssh.ClientConfig{
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			User:            sshUser,
			Auth: []ssh.AuthMethod{
				ssh.Password(sshPassword),
			},
			Timeout: time.Second,
		}

		sshConn, chans, reqs, err = ssh.NewClientConn(netConn, addr, sshConfig)
		if err != nil {
			return fmt.Errorf("failed to connect via SSH: %w", err)
		}

		return nil
	}, retry.Context(ctx),
		retry.Attempts(0),
		retry.DelayType(retry.FixedDelay),
		retry.Delay(time.Second),
	); err != nil {
		return nil, fmt.Errorf("failed to connect via SSH: %w", err)
	}

	return ssh.NewClient(sshConn, chans, reqs), nil
}
