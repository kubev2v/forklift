package copyappliance

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"time"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"golang.org/x/crypto/ssh"
	core "k8s.io/api/core/v1"
)

type SSHClient struct {
	Client     *ssh.Client
	conn       net.Conn
	User       string
	Address    string
	Port       string
	AuthMethod ssh.AuthMethod
}

func NewSSHClient(user string, address string, port string, secret *core.Secret) (client *SSHClient, err error) {
	if secret == nil {
		err = liberr.Wrap(errors.New("secret is nil"))
		return
	}
	key, found := secret.Data[sshPrivateKeyData]
	if !found {
		err = liberr.New(
			"the secret has no "+sshPrivateKeyData,
			"namespace", secret.Namespace,
			"name", secret.Name)
		return
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	authMethod := ssh.PublicKeys(signer)
	client = &SSHClient{
		User:       user,
		Address:    address,
		Port:       port,
		AuthMethod: authMethod,
	}
	return
}

// Connect logs in, and reports whether the far side answered at all. One that
// refuses the connection, or drops it before the handshake finishes, is still
// starting sshd: that is something to wait for, not a failure. One that answers
// and then rejects the key is a failure.
//
// The host key is not checked. The appliance VM is cloned fresh for each
// CopyAppliance, so there is no key recorded in advance to check it against;
// anything that can answer at this address can therefore impersonate it.
func (r *SSHClient) Connect(ctx context.Context) (ready bool, err error) {
	err = r.Close()
	if err != nil {
		return
	}
	config := &ssh.ClientConfig{
		User:            r.User,
		Auth:            []ssh.AuthMethod{r.AuthMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	addr := net.JoinHostPort(r.Address, r.Port)
	dialer := &net.Dialer{}
	netConn, dialErr := dialer.DialContext(ctx, "tcp", addr)
	if dialErr != nil {
		// Nothing is listening yet, or nothing answered. The guest reports its
		// address as soon as it has one, which is before sshd is accepting
		// connections.
		return false, nil //nolint:nilerr // dial failure means not ready yet
	}
	if deadline, ok := ctx.Deadline(); ok {
		// Getting connected is not finished until the handshake is, and an
		// appliance that accepts the socket and then says nothing would
		// otherwise hold it open for as long as the far side cares to. Cleared
		// again below, so that what the caller runs over the login is bounded
		// by SetTimeout and by nothing else.
		_ = netConn.SetDeadline(deadline)
	}
	cc, chans, reqs, err := ssh.NewClientConn(netConn, addr, config)
	if err != nil {
		_ = netConn.Close()
		if isStarting(err) {
			err = nil
			return
		}
		err = liberr.Wrap(err, "address", addr, "user", config.User)
		return
	}
	_ = netConn.SetDeadline(time.Time{})
	r.conn = netConn
	r.Client = ssh.NewClient(cc, chans, reqs)
	ready = true
	return
}

// RunCommand runs one command over the login. The command's output is carried
// into the error: a remote failure says nothing useful without it.
//
// A command that runs and exits non-zero is an error here. Whether that is a
// failure or the answer to a question only the caller knows, which is what
// IsExitError is for.
func (r *SSHClient) RunCommand(command string) (err error) {
	session, err := r.Client.NewSession()
	if err != nil {
		err = liberr.Wrap(err, "command", command)
		return
	}
	defer func() {
		_ = session.Close()
	}()
	output, err := session.CombinedOutput(command)
	if err != nil {
		err = liberr.Wrap(err, "command", command, "output", string(output))
		return
	}
	return
}

// RunWithStdin runs one command with the reader as its standard input, and
// returns once the far side has consumed all of it and the command has exited.
// The reader is streamed rather than read up front, so the payload never has to
// fit in the controller's memory.
//
// Standard error is collected into a buffer rather than read from a pipe: the
// pipe has to be drained before Wait, and a command that writes more than the
// channel window before exiting would deadlock against a Wait that never gets
// to run.
func (r *SSHClient) RunWithStdin(command string, in io.Reader) (err error) {
	session, err := r.Client.NewSession()
	if err != nil {
		err = liberr.Wrap(err, "command", command)
		return
	}
	defer func() {
		_ = session.Close()
	}()
	stderr := &bytes.Buffer{}
	session.Stderr = stderr

	stdin, err := session.StdinPipe()
	if err != nil {
		err = liberr.Wrap(err, "command", command)
		return
	}
	err = session.Start(command)
	if err != nil {
		err = liberr.Wrap(err, "command", command)
		return
	}
	// The command is told there is no more input either way. Closing is what
	// lets it finish, and a command that has already given up on us is better
	// reported by the exit status and the output it left behind than by the
	// broken pipe we saw writing to it.
	_, cErr := io.Copy(stdin, in)
	closeErr := stdin.Close()
	wErr := session.Wait()

	switch {
	case wErr != nil:
		// Wrapped rather than rebuilt, so that the exit status survives for
		// IsExitError to read.
		err = liberr.Wrap(wErr, "command", command, "output", stderr.String())
	case cErr != nil:
		err = liberr.Wrap(cErr, "command", command)
	case closeErr != nil:
		err = liberr.Wrap(closeErr, "command", command)
	}
	return
}

// SetTimeout bounds everything run over the login from here on. A caller that
// means to move real data has to ask for enough for all of it.
func (r *SSHClient) SetTimeout(timeout time.Duration) (err error) {
	err = r.conn.SetDeadline(time.Now().Add(timeout))
	return
}

// Close the login. One that was never connected, or has already been closed, is
// not an error.
func (r *SSHClient) Close() (err error) {
	if r.Client == nil {
		return
	}
	err = r.Client.Close()
	r.Client = nil
	r.conn = nil
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

// IsExitError reports whether err is a non-zero exit status from a command that ran.
func IsExitError(err error) (ok bool) {
	exited := &ssh.ExitError{}
	ok = errors.As(err, &exited)
	return
}

// isStarting reports whether a failed handshake means sshd is still coming up
// rather than refusing us. sshd accepts the socket before it is ready to talk,
// and drops the connection when it is not, so the handshake ends without a
// reply instead of with one.
func isStarting(err error) (ok bool) {
	ok = errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET)
	return
}
