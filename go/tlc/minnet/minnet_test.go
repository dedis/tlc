package minnet

import (
	"testing"
	"time"
	"fmt"
	"io"
	"bytes"
	"os"
	"net"
	"os/exec"
	"io/ioutil"
	"context"
	"crypto/x509"
	"crypto/rand"
	"crypto/elliptic"
	"crypto/ecdsa"
	"crypto/tls"
	"math/big"
	"encoding/gob"
	"encoding/pem"
	"encoding/json"
)

var multiProcess bool = true

func TestQSC(t *testing.T) {

	testCase(t, 1, 1, 100000, 0, 0)	// Trivial case: 1 of 1 consensus!
	testCase(t, 2, 2, 10000, 0, 0)	// Another trivial case: 2 of 2

	testCase(t, 2, 3, 1000, 0, 0)	// Standard f=1 case
	testCase(t, 3, 5, 1000, 0, 0)	// Standard f=2 case
	testCase(t, 4, 7, 100, 0, 0)	// Standard f=3 case
	testCase(t, 5, 9, 100, 0, 0)	// Standard f=4 case
	testCase(t, 11, 21, 20, 0, 0)	// Standard f=10 case
	//testCase(t, 101, 201, 10, 0, 0) // Standard f=100 case - blows up

	testCase(t, 3, 3, 1000, 0, 0)	// Larger-than-minimum thresholds
	testCase(t, 6, 7, 1000, 0, 0)
	testCase(t, 9, 10, 100, 0, 0)

	// Test with low-entropy tickets:
	// commit success rate will be bad, but still must remain safe!
	testCase(t, 2, 3, 1000, 1, 0)	// Limit case: will never commit
	testCase(t, 2, 3, 1000, 2, 0)	// Extreme low-entropy: rarely commits
	testCase(t, 2, 3, 1000, 3, 0)	// A bit better bit still bad...

	// Test with random delays inserted
	testCase(t, 2, 3, 1000, 0, 1 * time.Nanosecond)
	testCase(t, 2, 3, 1000, 0, 1 * time.Microsecond)
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

		if  multiProcess {
			testExec(t, threshold, nnodes)
		} else {
			testLocal(t, threshold, nnodes)
		}
	})
}

// Initialize and run the model for a given threshold and number of nodes.
func testLocal(t *testing.T, threshold, nnodes int) {
	//println("Run config", threshold, "of", nnodes)

	Threshold = threshold

	// Initialize the nodes
	all := make([]*Node, nnodes)
	for i := range all {
		all[i] = &Node{}
		all[i].self = i
		all[i].peer = make([]peer, nnodes)
		all[i].init()
	}
	for i := range all {
		for j := range all {
			rd, wr := io.Pipe()
			enc := gob.NewEncoder(wr)
			dec := gob.NewDecoder(rd)

			// Node i gets the write end of the pipe
			all[i].peer[j].wr = wr
			all[i].peer[j].enc = enc

			// Node j gets the read end
			all[j].peer[i].rd = rd
			all[j].peer[i].dec = dec
		}
	}

	// Run all the nodes asynchronously on separate goroutines
	for i, n := range all {
		n.done.Add(1)
		go n.runGossip(i)
	}

	// Wait for all the nodes to complete their execution
	for _, n := range all {
		n.done.Wait()
	}

	// Globally sanity-check and summarize each node's observed results
	for i, n := range all {
		commits := 0
		for s, committed := range n.commit {
			if committed {
				commits++
				for _, nn := range all {
					if nn.choice[s].From != n.choice[s].From {
						t.Fatalf("safety violation!" +
							"step %v", s)
					}
				}
			}
		}
		t.Logf("node %v committed %v of %v (%v%% success rate)",
			i, commits, len(n.commit), (commits*100)/len(n.commit))
	}
}

// Information passed to child processes via JSON
type testHost struct {
	Name	string		// Virtual host name
	Addr	string		// Host IP address and TCP port
}

func testExec(t *testing.T, threshold, nnodes int) {

	// Create a temporary directory for our key files
	tmpdir, err := ioutil.TempDir("", "tlc")
	if err != nil { t.Fatalf("TempDir: %v", err) }
	defer os.RemoveAll(tmpdir)	// clean up afterwards

	// Create a cancelable context in which to execute helper processes
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()			// kill child processes

	// Create a public/private keypair and self-signed cert for each node.
	host := make([]testHost, nnodes) // each node's host name and addr
	keyfn := make([]string, nnodes)	// each node's private key filename
	crtfn := make([]string, nnodes) // each node's certificate filename
	rootfn := fmt.Sprintf("%v/cert.pem", tmpdir)
	rootf, err := os.Create(rootfn)
	if err != nil {
		panic("Create: " + err.Error())
	}
	for i := range host {
		host[i].Name = fmt.Sprintf("host%v", i)

		certb, priv := createCert(host[i].Name)

		// Write the PEM-encoded cert to our root certs file
		if err := pem.Encode(rootf, &pem.Block{Type: "CERTIFICATE",
					Bytes: certb});  err != nil  {
			panic("pem.Encode: " + err.Error())
		}

		// PEM-encode this host's cert into a per-host cert file
		crtfn[i] = fmt.Sprintf("%v/cert-%v.pem", tmpdir, host[i].Name)
		crtf, err := os.Create(crtfn[i])
		if err != nil {
			panic("Create: " + err.Error())
		}
		if err := pem.Encode(crtf, &pem.Block{Type: "CERTIFICATE",
					Bytes: certb});  err != nil  {
			panic("pem.Encode: " + err.Error())
		}
		if err := crtf.Close(); err != nil {
			panic("Close: " + err.Error())
		}

		// PEM-encode this host's private key
		keyfn[i] = fmt.Sprintf("%v/key-%v.pem", tmpdir, host[i].Name)
		privb, err := x509.MarshalECPrivateKey(priv)
		if err != nil {
			t.Fatalf("x509.MarshalECPrivateKey: %v", err.Error())
		}
		keyf, err := os.Create(keyfn[i])
		if err != nil {
			panic("Create: " + err.Error())
		}
		if err := pem.Encode(keyf, &pem.Block{Type: "EC PRIVATE KEY",
					Bytes: privb}); err != nil {
			panic("pem.Encode: " + err.Error())
		}
		if err := keyf.Close(); err != nil {
			panic("Close: " + err.Error())
		}
	}
	if err := rootf.Close(); err != nil {
		panic("Close: " + err.Error())
	}

	// Start the per-node child processes
	cmd := make([]*exec.Cmd, nnodes)
	cin := make([]io.WriteCloser, nnodes)
	for i := range host {

		// Run the helper
		cmd[i] = exec.CommandContext(ctx, os.Args[0],
					"-test.run=TestHelper")
		cmd[i].Env = append(os.Environ(),
			fmt.Sprintf("TLC_HELPER_HOSTNAME=%v", host[i].Name),
			fmt.Sprintf("TLC_HELPER_KEYFILE=%v", keyfn[i]),
			fmt.Sprintf("TLC_HELPER_CERTFILE=%v", crtfn[i]),
			fmt.Sprintf("TLC_HELPER_ROOTFILE=%v", rootfn))

		// Arrange to send standard input to the child
		cin[i], err = cmd[i].StdinPipe()
		if err != nil {
			t.Fatalf("StdinPipe: %v", err.Error())
		}

		// Copy child's standard output to parent
		cmdout, err := cmd[i].StdoutPipe()
		if err != nil {
			t.Fatalf("StdoutPipe: %v", err.Error())
		}

		// Copy child's standard error to parent
		cmderr, err := cmd[i].StderrPipe()
		if err != nil {
			t.Fatalf("StderrPipe: %v", err.Error())
		}
		go copyAll(os.Stderr, cmderr)

		// Start the command running
		if err := cmd[i].Start(); err != nil {
			t.Fatalf("cmd.Start: %v", err.Error())
		}

		// Get the network address the child is listening on
		dec := json.NewDecoder(cmdout)
		if  err := dec.Decode(&host[i].Addr); err != nil {
			t.Fatalf("Copy: %v", err.Error())
		}
		//println("child", i, "listening on", host[i].Addr)
	}

	// Send the array of addresses to all the child processes
	for i := range host {
		enc := json.NewEncoder(cin[i])
		if err := enc.Encode(host); err != nil {
			t.Fatalf("Encode: " + err.Error())
		}
	}

	// Wait for the helper processes to complete
	for i := range host {
		if err := cmd[i].Wait(); err != nil {
			t.Fatalf("cmd.Wait: %v", err.Error())
		}
	}
}

func copyAll(dst io.Writer, src io.Reader) {
	if _, err := io.Copy(dst, src); err != nil {
		println("Copy: " + err.Error())
	}
}

func createCert(host string) ([]byte, *ecdsa.PrivateKey) {

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil { panic("createCert: " +err.Error()) }

	tmpl := x509.Certificate{
		IsCA: true,
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature |
			x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames: []string{host},
		SerialNumber: big.NewInt(1),
	}
	certb, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, 
						&priv.PublicKey, priv)
	if err != nil { panic("createCert: " +err.Error()) }

	return certb, priv
}

func TestHelper(t *testing.T) {
	hostName := os.Getenv("TLC_HELPER_HOSTNAME")
	keyfn := os.Getenv("TLC_HELPER_KEYFILE")
	crtfn := os.Getenv("TLC_HELPER_CERTFILE")
	rootfn := os.Getenv("TLC_HELPER_ROOTFILE")
	if hostName == "" {
		return	// Do nothing except when called as a helper
	}
	defer os.Exit(1)
	//println(hostName, "start")

	// Read our certificate and private key
	crt, err := tls.LoadX509KeyPair(crtfn, keyfn)
	if err != nil {
		panic("tls.LoadX509KeyPair: " + err.Error())
	}

	// Create a certificate pool containing all nodes' certificates
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(readFile(rootfn))

	// Configure TLS
	config := tls.Config{
		RootCAs: pool,
		Certificates: []tls.Certificate{crt},
		ServerName: hostName,
		//ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs: pool,
	}

	// Create a TLS/TCP listen socket
	tcpl, err := net.Listen("tcp", "")
	if err != nil {
		panic("Listen: " + err.Error())
	}

	// Listen and accept TLS connections
	go func() {
		for {
			// Accept a TCP connection
			tcpc, err := tcpl.Accept()
			if err != nil {
				panic("Accept: " + err.Error())
			}

			// Enable TLS on it and run the handshake.
			tlsc := tls.Server(tcpc, &config)
			if err := tlsc.Handshake(); err != nil {
				panic("Handshake: " + err.Error())
			}

			println("ACCEPTED:")
			io.Copy(os.Stderr, tlsc)
			panic("XXX DONE")
		}
	}()

	// Report the listener's network address to the parent process
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(tcpl.Addr().String()); err != nil {
		panic("Encode: " + err.Error())
	}

	// Get the JSON list of host names and addresses from the parent
	dec := json.NewDecoder(os.Stdin)
	host := []testHost{}
	if err := dec.Decode(&host); err != nil {
		panic("Decode: " + err.Error())
	}

	// Open TLS connections to each peer
	for i := range host {
		conn, err := tls.Dial("tcp", host[i].Addr, &config)
		if err != nil {
			panic("Dial: " + err.Error())
		}
		if err := conn.Handshake(); err != nil {
			panic("Handshake: " + err.Error())
		}

		conn.Write([]byte("FOOBAR"))
		io.Copy(os.Stderr, conn)
		conn.Close()
	}

	println(hostName, "done")
	os.Exit(0)
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

