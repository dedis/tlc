package minnet

import (
	"testing"
	"time"
	"fmt"
	"io"
	"bytes"
	"os"
	"net"
	"sync"
	"os/exec"
	"context"
	"crypto/x509"
	crand "crypto/rand"
	"crypto/elliptic"
	"crypto/ecdsa"
	"crypto/tls"
	"math/big"
	mrand  "math/rand"
	"encoding/gob"
	"encoding/pem"
	"encoding/json"
)

// Maximum random delays to add to message deliveries for testing
var MaxSleep time.Duration

// Whether to run consensus among multiple separate processes
var MultiProcess bool = true

// Whether to use TLS encryption and authentication atop TCP
var UseTLS bool = true


func TestQSC(t *testing.T) {

	testCase(t, 1, 1, 10000, 0, 0)	// Trivial case: 1 of 1 consensus!
	testCase(t, 2, 2, 10000, 0, 0)	// Another trivial case: 2 of 2

	testCase(t, 2, 3, 1000, 0, 0)	// Standard f=1 case
	testCase(t, 3, 5, 1000, 0, 0)	// Standard f=2 case
	testCase(t, 4, 7, 100, 0, 0)	// Standard f=3 case
	testCase(t, 5, 9, 100, 0, 0)	// Standard f=4 case
	testCase(t, 11, 21, 20, 0, 0)	// Standard f=10 case
	//testCase(t, 101, 201, 10, 0, 0) // Standard f=100 case - blows up

	testCase(t, 3, 3, 100, 0, 0)	// Larger-than-minimum thresholds
	testCase(t, 6, 7, 100, 0, 0)
	testCase(t, 9, 10, 100, 0, 0)

	// Test with low-entropy tickets:
	// commit success rate will be bad, but still must remain safe!
	testCase(t, 2, 3, 10, 1, 0)	// Limit case: will never commit
	testCase(t, 2, 3, 100, 2, 0)	// Extreme low-entropy: rarely commits
	testCase(t, 2, 3, 100, 3, 0)	// A bit better bit still bad...

	// Test with random delays inserted
	testCase(t, 2, 3, 100, 0, 1 * time.Nanosecond)
	testCase(t, 2, 3, 100, 0, 1 * time.Microsecond)
	testCase(t, 2, 3, 100, 0, 1 * time.Millisecond)
	testCase(t, 4, 7, 100, 0, 1 * time.Microsecond)
	testCase(t, 4, 7, 100, 0, 1 * time.Millisecond)
}

func testCase(t *testing.T, threshold, nnodes, maxSteps, maxTicket int,
		maxSleep time.Duration) {

	if maxTicket == 0 {		// Default to moderate-entropy tickets
		maxTicket = 10 * nnodes
	}

	desc := fmt.Sprintf("T=%v,N=%v,Steps=%v,Tickets=%v,Sleep=%v",
		threshold, nnodes, maxSteps, maxTicket, maxSleep)
	t.Run(desc, func(t *testing.T) {

		// Configure and run the test case.
		MaxSteps = maxSteps
		MaxTicket = int32(maxTicket)
		MaxSleep = maxSleep

		testExec(t, threshold, nnodes)
	})
}

// Information passed to child processes via JSON
type testHost struct {
	Name	string		// Virtual host name
	Addr	string		// Host IP address and TCP port
	Cert	[]byte		// Host's self-signed x509 certificate
}

func testExec(t *testing.T, threshold, nnodes int) {

	// Create a cancelable context in which to execute helper processes
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()			// kill child processes

	// Create a public/private keypair and self-signed cert for each node.
	conf := make([]testConfig, nnodes) // each node's config information
	for i := range conf {
		conf[i].Self = i
		conf[i].Nnodes = nnodes
		conf[i].HostName = fmt.Sprintf("host%v", i)
		conf[i].MaxSteps = MaxSteps
		conf[i].MaxTicket = MaxTicket
		conf[i].MaxSleep = MaxSleep
	}

	// Start the per-node child processes,
	// and gather network addresses and certificates from each one.
	childGroup := &sync.WaitGroup{}
	host := make([]testHost, nnodes)
	enc := make([]*json.Encoder, nnodes)
	dec := make([]*json.Decoder, nnodes)
	for i := range host {

		childGroup.Add(1)
		childIn, childOut := testExecChild(t, &conf[i], ctx, childGroup)

		// We'll communicate with the child via JSON-encoded stdin/out
		enc[i] = json.NewEncoder(childIn)
		dec[i] = json.NewDecoder(childOut)

		// Send the child its configuration information
		if err := enc[i].Encode(&conf[i]); err != nil {
			t.Fatalf("Encode: " + err.Error())
		}

		// Get the network address the child is listening on
		if  err := dec[i].Decode(&host[i]); err != nil {
			t.Fatalf("Decode: %v", err.Error())
		}
		if host[i].Name != conf[i].HostName {	// sanity check
			panic("hostname mismatch")
		}
		//println("child", i, "listening on", host[i].Addr)
	}

	// Send the array of addresses to all the child processes
	for i := range host {
		if err := enc[i].Encode(host); err != nil {
			t.Fatalf("Encode: " + err.Error())
		}
	}

	// Wait and collect the consensus histories of each child
	hist := make([][]choice, nnodes)
	for i := range host {
		if  err := dec[i].Decode(&hist[i]); err != nil {
			t.Fatalf("Decode: %v", err.Error())
		}
	}

	// Let all the children know they can exit
	for i := range host {
		if err := enc[i].Encode(struct{}{}); err != nil {
			t.Fatalf("Encode: " + err.Error())
		}
	}

	// Wait for the helper processes to complete
	childGroup.Wait()
}

// Exec a child as a separate process.
func testExecChild(t *testing.T, conf *testConfig, ctx context.Context,
			grp *sync.WaitGroup) (io.Writer, io.Reader) {

	if !MultiProcess {
		// Run a child as a separate goroutine in the same process.
		childInRd, childInWr := io.Pipe()
		childOutRd, childOutWr := io.Pipe()
		go func() {
			testChild(childInRd, childOutWr)
			grp.Done()
		}()
		return childInWr, childOutRd
	}

	// Run the child as a separate helper process
	cmd := exec.CommandContext(ctx, os.Args[0],
				"-test.run=TestHelper")
	cmd.Env = append(os.Environ(), "TLC_HELPER=1")

	// Arrange to send standard input to the child via pipe
	childIn, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err.Error())
	}

	// Copy child's standard output to parent via pipe
	childOut, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err.Error())
	}

	// Copy child's standard error to parent's standard error
	childErr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("StderrPipe: %v", err.Error())
	}
	go copyAll(os.Stderr, childErr)

	// Start the command running
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err.Error())
	}

	// Arrange to signal the provided WaitGroup when child terminates
	go func() {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("cmd.Wait: %v", err.Error())
		}
		grp.Done()
	}()

	return childIn, childOut
}

func TestHelper(t *testing.T) {

	if os.Getenv("TLC_HELPER") == "" {
		return	// Do nothing except when called as a helper
	}

	// Exit with error status if anything goes wrong.
	defer os.Exit(1)

	testChild(os.Stdin, os.Stdout)
	os.Exit(0)
}

func copyAll(dst io.Writer, src io.Reader) {
	if _, err := io.Copy(dst, src); err != nil {
		println("Copy: " + err.Error())
	}
}

func createCert(hostName string) (certPemBytes, privPemBytes []byte) {

	priv, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil { panic("createCert: " +err.Error()) }

	notBefore := time.Now()				// valid starting now
	notAfter := notBefore.Add(365*24*time.Hour)	// valid for a year
	tmpl := x509.Certificate{
		NotBefore: notBefore,
		NotAfter: notAfter,
		IsCA: true,
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature |
			x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth,
						x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames: []string{hostName},
		SerialNumber: big.NewInt(1),
	}
	certb, err := x509.CreateCertificate(crand.Reader, &tmpl, &tmpl, 
						&priv.PublicKey, priv)
	if err != nil { panic("createCert: " + err.Error()) }

	cert, err := x509.ParseCertificate(certb)
	if err != nil { panic("ParseCertificate: " + err.Error()) }

	if err := cert.VerifyHostname(hostName); err != nil {
		panic("VerifyHostname: " + err.Error())
	}

	// Sanity-check the certificate just to make sure it actually works.
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	vo := x509.VerifyOptions{ DNSName: hostName, Roots: pool }
	if _, err := cert.Verify(vo); err != nil {
		panic("Verify: " + err.Error())
	}
	//println("verified for", hostName)

	// PEM-encode our certificate
	certPem := bytes.NewBuffer(nil)
	if err := pem.Encode(certPem, &pem.Block{Type: "CERTIFICATE",
				Bytes: certb});  err != nil  {
		panic("pem.Encode: " + err.Error())
	}

	// PEM-encode our private key
	privb, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		panic("x509.MarshalECPrivateKey: " + err.Error())
	}
	privPem := bytes.NewBuffer(nil)
	if err := pem.Encode(privPem, &pem.Block{Type: "EC PRIVATE KEY",
				Bytes: privb}); err != nil {
		panic("pem.Encode: " + err.Error())
	}

	return certPem.Bytes(), privPem.Bytes()
}

func testChild(in io.Reader, out io.Writer) {

	// We'll use JSON over stdin/stdout to coordinate with our parent.
	dec := json.NewDecoder(in)
	enc := json.NewEncoder(out)

	// Get the child process config information via JSON
	conf := testConfig{}
	if err := dec.Decode(&conf); err != nil {
		panic("Decode: " + err.Error())
	}
	self := conf.Self
	MaxSteps = conf.MaxSteps
	MaxTicket = conf.MaxTicket
	MaxSleep = conf.MaxSleep

	// Initialize the node appropriately
	//println("self", self, "nnodes", conf.Nnodes)
	n := &Node{}
	n.init(self, make([]peer, conf.Nnodes))
	n.mutex.Lock()	// keep node's TLC state locked until fully set up

	// Create a TLS/TCP listen socket for this child
	tcpl, err := net.Listen("tcp", "")
	if err != nil {
		panic("Listen: " + err.Error())
	}

	// Create an x509 certificate and private key for this child
	//println(self, "createCert for", conf.HostName)
	certb, privb := createCert(conf.HostName)

	// Create a TLS certificate from it
	tlscert, err := tls.X509KeyPair(certb, privb)
	if err != nil {
		panic("tls.X509KeyPair: " + err.Error())
	}

	// Report our network address and certificate to the parent process
	myHost := testHost{
			Name: conf.HostName,
			Addr: tcpl.Addr().String(),
			Cert: certb,
		}
	if err := enc.Encode(myHost); err != nil {
		panic("Encode: " + err.Error())
	}

	// Get the list of all host names, addresses, and certs from the parent
	host := []testHost{}
	if err := dec.Decode(&host); err != nil {
		panic("Decode: " + err.Error())
	}

	// Create a certificate pool containing all nodes' certificates
	pool := x509.NewCertPool()
	for i := range host {
		if !pool.AppendCertsFromPEM(host[i].Cert) {
			panic("failed to append cert from " + host[i].Name)
		}
	}

	// Configure TLS
	tlsConf := &tls.Config{
		RootCAs: pool,
		Certificates: []tls.Certificate{tlscert},
		ServerName: conf.HostName,
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs: pool,
	}
	//println("hostName", conf.HostName, "pool", len(pool.Subjects()))

	// Listen and accept TCP/TLS connections
	donegrp := &sync.WaitGroup{}
	go func() {
		for {
			// Accept a TCP connection
			tcpc, err := tcpl.Accept()
			if err != nil {
				panic("Accept: " + err.Error())
			}

			// Launch a goroutine to process it
			donegrp.Add(1)
			go n.acceptNetwork(tcpc, tlsConf, host, donegrp)
		}
	}()

	// Open TCP and optionally TLS connections to each peer
	//println(self, "open TLS connections to", len(host), "peers")
	stepgrp := &sync.WaitGroup{}
	for i := range host {
		// Open an authenticated TLS connection to peer i
		peerConf := *tlsConf
		peerConf.ServerName = host[i].Name
		//println(self, "Dial", host[i].Name, host[i].Addr)
		var conn net.Conn
		if UseTLS {
			conn, err = tls.Dial("tcp", host[i].Addr, &peerConf)
		} else {
			conn, err = net.Dial("tcp", host[i].Addr)
		}
		if err != nil {
			panic("Dial: " + err.Error())
		}

		// Tell the server which client we are.
		enc := gob.NewEncoder(conn)
		if err := enc.Encode(self); err != nil {
			panic("gob.Encode: " + err.Error())
		}

		// Set up a peer sender object.
		// It signals stepgrp.Done() after enough steps pass.
		stepgrp.Add(1)
		n.peer[i] = &testPeer{ enc, stepgrp, conn }
	}
	//println(self, "opened TLS connections")

	// Start the consensus test
	n.advanceTLC(0)

	// Now we can let the receive goroutines process incoming messages
	n.mutex.Unlock()

	// Wait to finish enough consensus rounds
	//println(self, "wait for test to complete")
	stepgrp.Wait()

	// Report our observed consensus history to the parent
	if err := enc.Encode(n.choice); err != nil {
		panic("Encode: " + err.Error())
	}

	// Finally, wait for our parent to signal when the test is complete.
	if err := dec.Decode(&struct{}{}); err != nil {
		panic("Decode: " + err.Error())
	}

	//println(self, "child finished")
}

func writeFile(name string, data []byte) {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic("OpenFile: %v" + err.Error())
	}
	_, err = f.Write(data)
	if err != nil {
		panic("Write: %v" + err.Error())
	}
	err =  f.Close()
	if err != nil {
		panic("Close: %v" + err.Error())
	}
}

func readFile(name string) []byte {
	f, err := os.Open(name)
	if err != nil {
		panic("OpenFile: %v" + err.Error())
	}
	buf := bytes.NewBuffer(nil)
	if _, err := buf.ReadFrom(f); err != nil {
		panic("ReadFrom: %v" + err.Error())
	}
	err =  f.Close()
	if err != nil {
		panic("Close: %v" + err.Error())
	}
	return buf.Bytes()
}

// Accept a new TLS connection on a TCP server socket.
func (n *Node) acceptNetwork(conn net.Conn, tlsConf *tls.Config,
				host []testHost, donegrp *sync.WaitGroup) {

	// Enable TLS on the connection and run the handshake.
	if UseTLS {
		conn = tls.Server(conn, tlsConf)
	}
	defer func() { conn.Close() }()

	// Receive the client's nodenumber indication
	dec := gob.NewDecoder(conn)
	var peer int
	if err := dec.Decode(&peer); err != nil {
		println(n.self, "acceptNetwork gob.Decode: " + err.Error())
		return
		//panic("acceptNetwork gob.Decode: " + err.Error())
	}
	if peer < 0 || peer >= len(host) {
		println("acceptNetwork: bad peer number")
		return
	}

	// Authenticate the client with TLS.
	// XXX Why doesn't VerifyHostname work to verify a client auth?
	// Go TLS bug to report?
	//if err := tlsc.VerifyHostname(host[peer].Name); err != nil {
	//	panic("VerifyHostname: " + err.Error())
	//}
	if UseTLS {
		cs := conn.(*tls.Conn).ConnectionState()
		if len(cs.PeerCertificates) < 1 {
			println("acceptNetwork: no certificate from client")
			return
		}
		err := cs.PeerCertificates[0].VerifyHostname(host[peer].Name)
		if err != nil {
			println("VerifyHostname: " + err.Error())
			return
		}
	}

	// Receive and process arriving messages
	n.runReceiveNetwork(peer, dec, donegrp)
}

// Receive messages from a connection and dispatch them into the TLC stack.
func (n *Node) runReceiveNetwork(peer int, dec *gob.Decoder,
				grp *sync.WaitGroup) {
	for  {
		// Get next message from this peer
		msg := Message{}
		err := dec.Decode(&msg)
		if err == io.EOF {
			break
		} else if err != nil {
			panic("receiveGossip:" + err.Error())
		}
		//println(n.self, n.tmpl.Step, "runReceiveNetwork: recv from",
		//	msg.From, "type", msg.Typ, "seq", msg.Seq,
		//	"step", msg.Step)

		// Optionally insert random delays on a message basis
		time.Sleep(time.Duration(mrand.Int63n(int64(MaxSleep+1))))

		grp.Add(1)
		go n.receiveNetwork(&msg, grp)
	}
	grp.Done()	// signal that we're done
}

func (n *Node) receiveNetwork(msg *Message, grp *sync.WaitGroup) {

	// Keep the stack single-threaded.
	n.mutex.Lock()
	defer func() {
		n.mutex.Unlock()
		grp.Done()
	}()

	// Dispatch up to the gossip layer
	//println(n.self, n.tmpl.Step, "receiveNetwork from", msg.From,
	//	"type", msg.Typ,  "seq", msg.Seq, "vec", len(msg.Vec))
	n.receiveGossip(msg)
}


// Configuration information each child process needs to launch
type testConfig struct {
	Self	int		// Which participant number we are
	Nnodes	int		// Total number of participants
	HostName string		// This child's virtual hostname

	MaxSteps int
	MaxTicket int32
	MaxSleep time.Duration
}


type testPeer struct {
	e *gob.Encoder
	w *sync.WaitGroup
	c io.Closer
}

func (tp *testPeer) Send(msg *Message) {
	if tp.e != nil {
		//println("testPeer.Send seq", msg.Seq, "step", msg.Step,
		//	"MaxSteps", MaxSteps)
		if err := tp.e.Encode(msg); err != nil {
			println("Encode:", err.Error())
		}
	}
	if tp.w != nil && MaxSteps > 1 && msg.Step >= MaxSteps {
		//println("testPeer.Send done")
		tp.w.Done()
		tp.w = nil
	}
}

