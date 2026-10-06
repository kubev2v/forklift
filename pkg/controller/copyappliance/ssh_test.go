package copyappliance

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"golang.org/x/crypto/ssh"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The appliance is talked to over a real protocol, so the tests answer it with
// a real one. An SSH server standing in for the appliance is a handful of lines
// and exercises the handshake, the auth exchange and the exec channel the way
// the appliance will; a stub behind an interface would only replay what the
// production code already assumes.
type sshServer struct {
	addr string
	// failing names the commands the appliance refuses to run, and dropping
	// the ones it drops the whole connection on rather than answering.
	failing  map[string]bool
	dropping map[string]bool
	mutex    sync.Mutex
	ran      []sshCommand
}

// sshCommand is one command the appliance was asked to run and everything it
// was given on its standard input.
type sshCommand struct {
	command string
	stdin   []byte
}

// commandFailureOutput is what a failing command writes, so that a test can see
// whether the output made it into the error.
const commandFailureOutput = "no such file or directory"

// startSSHServer answers on the loopback address until the test ends, accepting
// the one key it is given and refusing every other. A nil key refuses all of
// them, which is the appliance that was built with somebody else's.
func startSSHServer(t *testing.T, authorized ssh.PublicKey, failing ...string) *sshServer {
	t.Helper()
	server := &sshServer{
		failing:  map[string]bool{},
		dropping: map[string]bool{},
	}
	for _, command := range failing {
		server.failing[command] = true
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, offered ssh.PublicKey) (*ssh.Permissions, error) {
			if authorized == nil || !bytes.Equal(offered.Marshal(), authorized.Marshal()) {
				return nil, fmt.Errorf("key not in authorized_keys")
			}
			return &ssh.Permissions{}, nil
		},
	}
	config.AddHostKey(testSigner(t))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	server.addr = listener.Addr().String()

	go func() {
		for {
			conn, aErr := listener.Accept()
			if aErr != nil {
				// The listener was closed at the end of the test.
				return
			}
			go server.serve(conn, config)
		}
	}()
	return server
}

// dropOn makes the appliance drop the connection when it is asked to run this
// command, which is the network going away part way through a transfer.
func (r *sshServer) dropOn(command string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.dropping[command] = true
}

// Ran is the commands the appliance was asked to run, in the order it was asked.
func (r *sshServer) Ran() []string {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	names := make([]string, 0, len(r.ran))
	for _, ran := range r.ran {
		names = append(names, ran.command)
	}
	return names
}

// Stdin is what the named command was given on its standard input, and whether
// it was run at all. The last run wins, which for a command run once is the
// only one there is.
func (r *sshServer) Stdin(command string) (stdin []byte, ran bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, was := range slices.Backward(r.ran) {
		if was.command == command {
			return was.stdin, true
		}
	}
	return
}

func (r *sshServer) serve(conn net.Conn, config *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		// A rejected key, or a client that hung up. Either way there is no
		// connection left to serve.
		_ = conn.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, newChannel.ChannelType())
			continue
		}
		channel, requests, aErr := newChannel.Accept()
		if aErr != nil {
			return
		}
		go r.session(channel, requests, func() { _ = conn.Close() })
	}
}

// session answers one exec request and closes, which is what one command over
// its own session looks like from the appliance's side.
func (r *sshServer) session(channel ssh.Channel, requests <-chan *ssh.Request, drop func()) {
	defer func() { _ = channel.Close() }()
	for request := range requests {
		if request.Type != "exec" {
			if request.WantReply {
				_ = request.Reply(false, nil)
			}
			continue
		}
		var payload struct{ Command string }
		_ = ssh.Unmarshal(request.Payload, &payload)
		if request.WantReply {
			_ = request.Reply(true, nil)
		}
		status, dropped := r.run(payload.Command, channel)
		if dropped {
			drop()
			return
		}
		_, _ = channel.SendRequest(
			"exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
		return
	}
}

// run reads the command's standard input to the end and then answers. Draining
// first is what a real command does, and a server that replied without draining
// would hang any client sending more than fits in the channel's window.
func (r *sshServer) run(command string, channel ssh.Channel) (status uint32, dropped bool) {
	stdin, _ := io.ReadAll(channel)
	r.mutex.Lock()
	r.ran = append(r.ran, sshCommand{command: command, stdin: stdin})
	refused := r.failing[command]
	dropped = r.dropping[command]
	r.mutex.Unlock()
	if dropped {
		return
	}
	if refused {
		// On stderr, where a failing command writes. CombinedOutput merges the
		// two, so a caller that reads either still sees it.
		_, _ = io.WriteString(channel.Stderr(), commandFailureOutput)
		status = 1
	}
	return
}

// testSigner is a host key. ed25519 because it generates instantly; an RSA key
// would take longer than everything else in the package put together.
func testSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatalf("host key signer: %v", err)
	}
	return signer
}

// testKeyPair is a private key in the PEM form the secret holds, and the public
// half the appliance image would have installed.
func testKeyPair(t *testing.T) (private []byte, public ssh.PublicKey) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(key, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("key signer: %v", err)
	}
	return pem.EncodeToMemory(block), signer.PublicKey()
}

// sshContext is an appliance reachable at the given address. Nothing here talks
// to vCenter, so it has no connection. The address's port is ignored: production
// always dials ApplianceSSHPort, and tests that need a live SSH session use
// NewSSHClient with the test server's port directly.
func sshContext(t *testing.T, private []byte, addr string) *ApplianceContext {
	t.Helper()
	appliance := testAppliance()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split %q: %v", addr, err)
	}
	appliance.Status.Addresses = []api.ApplianceAddress{
		{Network: "VM Network", MAC: "00:50:56:01:02:03", IP: host},
	}
	// One secret carries both halves, so an appliance without a key still has
	// the certificates the steps ahead of the login read.
	data := make(map[string][]byte, len(applianceTLS().data)+1)
	for key, value := range applianceTLS().data {
		data[key] = value
	}
	if private != nil {
		data[sshPrivateKeyData] = private
	}
	return &ApplianceContext{
		Appliance: appliance,
		Log:       testLog(),
		ApplianceSecret: &core.Secret{
			ObjectMeta: meta.ObjectMeta{
				Namespace: appliance.Spec.Secret.Namespace,
				Name:      appliance.Spec.Secret.Name,
			},
			Data: data,
		},
	}
}

// errorMentions reports whether the error says the given thing anywhere an
// operator would read it. liberr keeps the message fixed and carries what names
// the secret, the command or the network beside it, so both have to be looked
// at.
func errorMentions(t *testing.T, err error, want string) bool {
	t.Helper()
	if strings.Contains(err.Error(), want) {
		return true
	}
	wrapped := &liberr.Error{}
	if !errors.As(err, &wrapped) {
		t.Fatalf("error = %v (%T), want a liberr.Error", err, err)
	}
	for _, value := range wrapped.Context() {
		if strings.Contains(fmt.Sprintf("%v", value), want) {
			return true
		}
	}
	return false
}

// closedAddr is an address that accepted a connection a moment ago and does not
// now, which is the appliance between the guest reporting its address and sshd
// coming up.
func closedAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return addr
}

func TestSSHClient(t *testing.T) {
	t.Run("a login with the installed key succeeds", func(t *testing.T) {
		private, public := testKeyPair(t)
		server := startSSHServer(t, public)
		client := loginAt(t, private, server.addr)

		if err := client.RunCommand("true"); err != nil {
			t.Fatalf("RunCommand: %v", err)
		}
	})

	// An appliance that answers and then turns us away is not one to wait for.
	t.Run("a login with a key the appliance does not know fails", func(t *testing.T) {
		private, _ := testKeyPair(t)
		_, installed := testKeyPair(t)
		server := startSSHServer(t, installed)
		host, port, err := net.SplitHostPort(server.addr)
		if err != nil {
			t.Fatalf("split: %v", err)
		}
		withSettings(t, testSettings())
		secret := &core.Secret{Data: map[string][]byte{sshPrivateKeyData: private}}
		client, err := NewSSHClient(Settings.SSHUser, host, port, secret)
		if err != nil {
			t.Fatalf("NewSSHClient: %v", err)
		}
		ready, err := client.Connect(context.TODO())
		if err == nil {
			t.Fatal("Connect succeeded with a key the appliance does not know")
		}
		if ready {
			t.Error("ready = true, want a rejected key reported as not ready")
		}
	})

	t.Run("an address nothing is listening on has not answered", func(t *testing.T) {
		private, _ := testKeyPair(t)
		host, port, err := net.SplitHostPort(closedAddr(t))
		if err != nil {
			t.Fatalf("split: %v", err)
		}
		withSettings(t, testSettings())
		secret := &core.Secret{Data: map[string][]byte{sshPrivateKeyData: private}}
		client, err := NewSSHClient(Settings.SSHUser, host, port, secret)
		if err != nil {
			t.Fatalf("NewSSHClient: %v", err)
		}
		ready, err := client.Connect(context.TODO())
		if err != nil {
			t.Fatalf("Connect: %v, want a closed port to be something to wait for", err)
		}
		if ready {
			t.Error("ready = true, want a closed port reported as not ready")
		}
	})

	// The provider's key secrets carry both halves under fixed names, so a
	// secret without the private one is the wrong secret.
	t.Run("a secret with no private key fails", func(t *testing.T) {
		ac := sshContext(t, nil, closedAddr(t))
		ac.ApplianceSecret = &core.Secret{
			ObjectMeta: meta.ObjectMeta{Namespace: "forklift", Name: "appliance-secret"},
			Data:       map[string][]byte{"public-key": []byte("ssh-ed25519 AAAA")},
		}

		_, _, err := ac.SSHClient(context.TODO(), sshTimeout)
		if err == nil {
			t.Fatal("SSHClient succeeded with no private key in the secret")
		}
		if !errorMentions(t, err, sshPrivateKeyData) {
			t.Errorf("error = %q, want it to name %q", err, sshPrivateKeyData)
		}
	})

	// The secret is tolerated as missing so that a teardown is not blocked, so
	// this is where an operator finds out which one to create.
	t.Run("a missing secret fails by name", func(t *testing.T) {
		ac := sshContext(t, nil, closedAddr(t))
		ac.ApplianceSecret = nil

		_, _, err := ac.SSHClient(context.TODO(), sshTimeout)
		if err == nil {
			t.Fatal("SSHClient succeeded with no secret at all")
		}
		if !errorMentions(t, err, ac.Appliance.Spec.Secret.Name) {
			t.Errorf("error = %q, want it to name the secret", err)
		}
	})
}

// The timeout SSHClient is given is what the caller is prepared to spend on the
// transfer the login is for, and LoadImage asks for thirty minutes of it. The
// reconcile's own context has to be able to end it sooner: otherwise one
// appliance that accepts connections and then says nothing holds a reconcile
// worker for the whole half hour.
func TestSSHClientHonoursTheContextDeadline(t *testing.T) {
	private, _ := testKeyPair(t)
	addr := silentAddr(t)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	secret := &core.Secret{Data: map[string][]byte{sshPrivateKeyData: private}}
	client, err := NewSSHClient(Settings.SSHUser, host, port, secret)
	if err != nil {
		t.Fatalf("NewSSHClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.TODO(), 250*time.Millisecond)
	defer cancel()

	type result struct {
		ready bool
		err   error
	}
	finished := make(chan result, 1)
	go func() {
		ready, err := client.Connect(ctx)
		finished <- result{ready, err}
	}()

	select {
	case got := <-finished:
		if got.err == nil {
			t.Fatal("Connect succeeded against an appliance that never said anything")
		}
		if got.ready {
			t.Error("ready = true, want neither")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Connect is still waiting: the login is bounded by its own timeout only")
	}
}

// silentAddr accepts connections and then says nothing, which is the appliance
// that costs the most: the dial succeeds, so there is no refusal to read as
// "still starting", and the handshake waits for a banner that never comes.
func silentAddr(t *testing.T) (addr string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	t.Cleanup(func() {
		close(done)
		_ = listener.Close()
	})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				<-done
				_ = conn.Close()
			}()
		}
	}()
	return listener.Addr().String()
}

func TestRunCommand(t *testing.T) {
	t.Run("the command runs on the appliance", func(t *testing.T) {
		_, server, client := applianceLogin(t)

		err := client.RunCommand("configure")
		if err != nil {
			t.Fatalf("RunCommand: %v", err)
		}
		want := []string{"configure"}
		if !slices.Equal(server.Ran(), want) {
			t.Errorf("ran %v, want %v", server.Ran(), want)
		}
	})

	// "Process exited with status 1" on its own tells an operator nothing.
	t.Run("a failed command carries its output into the error", func(t *testing.T) {
		_, _, client := applianceLogin(t, "configure")

		err := client.RunCommand("configure")
		if err == nil {
			t.Fatal("RunCommand succeeded with a failing command")
		}
		if !errorMentions(t, err, commandFailureOutput) {
			t.Errorf("error = %q, want it to carry %q", err, commandFailureOutput)
		}
		if !errorMentions(t, err, "configure") {
			t.Errorf("error = %q, want it to name the command", err)
		}
	})
}

// applianceLogin is a logged-in client on an appliance that refuses the named
// commands.
func applianceLogin(t *testing.T, failing ...string) (*ApplianceContext, *sshServer, *SSHClient) {
	t.Helper()
	private, public := testKeyPair(t)
	server := startSSHServer(t, public, failing...)
	ac := sshContext(t, private, server.addr)
	client := loginAt(t, private, server.addr)
	return ac, server, client
}

// loginAt opens an SSH session to addr with the given private key. Tests use
// this instead of ApplianceContext.SSHClient so they can reach a listener on a
// non-standard port without a production injection hook.
func loginAt(t *testing.T, private []byte, addr string) *SSHClient {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split %q: %v", addr, err)
	}
	withSettings(t, testSettings())
	secret := &core.Secret{Data: map[string][]byte{sshPrivateKeyData: private}}
	client, err := NewSSHClient(Settings.SSHUser, host, port, secret)
	if err != nil {
		t.Fatalf("NewSSHClient: %v", err)
	}
	ready, err := client.Connect(context.TODO())
	if err != nil || !ready {
		t.Fatalf("Connect: (%v, %v)", ready, err)
	}
	if err := client.SetTimeout(sshTimeout); err != nil {
		t.Fatalf("SetTimeout: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestRunWithStdin(t *testing.T) {
	t.Run("the payload arrives whole and unaltered", func(t *testing.T) {
		_, server, client := applianceLogin(t)
		payload := []byte("\x00\x01 a payload with an embedded NUL and a \n in it\xff")

		err := client.RunWithStdin("load", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("RunWithStdin: %v", err)
		}
		got, ran := server.Stdin("load")
		if !ran {
			t.Fatal("the command was never run")
		}
		if !bytes.Equal(got, payload) {
			t.Errorf("stdin = %q, want %q", got, payload)
		}
	})

	// The image is hundreds of megabytes, which is many times the SSH channel
	// window. Writing it has to keep pace with the far side reading it rather
	// than filling the window and stopping.
	t.Run("a payload larger than one channel window does not stall", func(t *testing.T) {
		_, server, client := applianceLogin(t)
		payload := make([]byte, 4<<20)
		if _, err := rand.Read(payload); err != nil {
			t.Fatalf("generate payload: %v", err)
		}

		err := client.RunWithStdin("load", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("RunWithStdin: %v", err)
		}
		got, _ := server.Stdin("load")
		if !bytes.Equal(got, payload) {
			t.Errorf("stdin is %d bytes, want the %d that were sent", len(got), len(payload))
		}
	})

	// "Process exited with status 1" on its own tells an operator nothing, and
	// this is the one command whose failure they will have to act on.
	t.Run("a failed command carries its output into the error", func(t *testing.T) {
		_, _, client := applianceLogin(t, "load")

		err := client.RunWithStdin("load", strings.NewReader("payload"))
		if err == nil {
			t.Fatal("RunWithStdin succeeded against a command that failed")
		}
		if !errorMentions(t, err, commandFailureOutput) {
			t.Errorf("error = %q, want it to carry %q", err, commandFailureOutput)
		}
		if !errorMentions(t, err, "load") {
			t.Errorf("error = %q, want it to name the command", err)
		}
	})
}

// A command the caller asked as a question has two answers, and both of them
// come back from RunCommand the same way a lost connection does. IsExitError is
// what tells them apart: reading "no" as a failure would fail every deploy that
// has not loaded its image yet, which is all of them, and reading a lost
// connection as "no" would have the caller act on an answer nobody gave.
func TestIsExitError(t *testing.T) {
	const command = "podman image exists something"

	t.Run("a command that exits zero has nothing to classify", func(t *testing.T) {
		_, _, client := applianceLogin(t)

		err := client.RunCommand(command)

		if err != nil {
			t.Fatalf("RunCommand: %v", err)
		}
	})

	t.Run("a command that exits non-zero has answered", func(t *testing.T) {
		_, _, client := applianceLogin(t, command)

		err := client.RunCommand(command)

		if err == nil {
			t.Fatal("RunCommand succeeded against a command that failed")
		}
		if !IsExitError(err) {
			t.Errorf("IsExitError(%v) = false, want a non-zero exit read as an answer", err)
		}
	})

	// Dropped rather than closed from this side: the appliance going away part
	// way through is what the caller has to tell from an answer, and a link this
	// side has already given up on is not that.
	t.Run("a connection that has gone away is not an answer", func(t *testing.T) {
		_, server, client := applianceLogin(t)
		server.dropOn(command)

		err := client.RunCommand(command)

		if err == nil {
			t.Fatal("RunCommand succeeded over a connection that went away")
		}
		if IsExitError(err) {
			t.Errorf("IsExitError(%v) = true, want no answer at all", err)
		}
	})
}

// The setup pod installs the supervisor over a login it already holds, so a
// missing secret has to be caught before the pod is created rather than after.
func TestConnectApplianceRequiresASecret(t *testing.T) {
	private, _ := testKeyPair(t)
	ac := sshContext(t, private, closedAddr(t))
	ac.ApplianceSecret = nil

	_, err := connectAppliance(context.TODO(), ac)

	if err == nil {
		t.Fatal("connectAppliance succeeded with no key to log in with")
	}
}
