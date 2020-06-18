package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"storj.io/uplink"
)

func main() {
	apikey := js.Global().Get("apikey").String()
	satellite := js.Global().Get("satellite").String()
	passphrase := js.Global().Get("passphrase").String()
	log("Found apikey=%s", apikey)

	err := UploadAndDownloadData(context.Background(), satellite, apikey, passphrase,
		"my-first-bucket", "foo/bar/baz", []byte("one fish two fish red fish blue fish"))
	if err != nil {
		fmt.Println("error:", err)
	}
	//run indefinitely
	select {}
}

func log(format string, a ...interface{}) {
	js.Global().Get("console").Call("log", fmt.Sprintf(format, a...))
}

// UploadAndDownloadData uploads the specified data to the specified key in the
// specified bucket, using the specified Satellite, API key, and passphrase.
func UploadAndDownloadData(ctx context.Context,
	satelliteAddress, apiKey, passphrase, bucketName, uploadKey string,
	dataToUpload []byte) error {

	// Request access grant to the satellite with the API key and passphrase.
	myConfig := uplink.Config{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			fmt.Println("dial context")
			addressParts := strings.Split(address, ":")
			port, _ := strconv.Atoi(addressParts[1])
			return NewJsConn(addressParts[0], port)
		},
	}
	access, err := myConfig.RequestAccessWithPassphrase(ctx, satelliteAddress, apiKey, passphrase)
	if err != nil {
		return fmt.Errorf("could not request access grant: %v", err)
	}
	fmt.Println("\n\n>>> access grant requested successfully <<<\n")
	/*
		access, err := uplink.RequestAccessWithPassphrase(ctx, satelliteAddress, apiKey, passphrase)
		if err != nil {
			return fmt.Errorf("could not request access grant: %v", err)
		}
	*/

	// Open up the Project we will be working with.
	project, err := myConfig.OpenProject(ctx, access)
	if err != nil {
		return fmt.Errorf("could not open project: %v", err)
	}
	defer project.Close()

	// Ensure the desired Bucket within the Project is created.
	_, err = project.EnsureBucket(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("could not ensure bucket: %v", err)
	}

	// Intitiate the upload of our Object to the specified bucket and key.
	upload, err := project.UploadObject(ctx, bucketName, uploadKey, nil)
	if err != nil {
		return fmt.Errorf("could not initiate upload: %v", err)
	}

	// Copy the data to the upload.
	buf := bytes.NewBuffer(dataToUpload)
	_, err = io.Copy(upload, buf)
	if err != nil {
		_ = upload.Abort()
		return fmt.Errorf("could not upload data: %v", err)
	}

	// Commit the uploaded object.
	err = upload.Commit()
	if err != nil {
		return fmt.Errorf("could not commit uploaded object: %v", err)
	}

	// Initiate a download of the same object again
	download, err := project.DownloadObject(ctx, bucketName, uploadKey, nil)
	if err != nil {
		return fmt.Errorf("could not open object: %v", err)
	}
	defer download.Close()

	// Read everything from the download stream
	receivedContents, err := ioutil.ReadAll(download)
	if err != nil {
		return fmt.Errorf("could not read data: %v", err)
	}

	// Check that the downloaded data is the same as the uploaded data.
	if !bytes.Equal(receivedContents, dataToUpload) {
		return fmt.Errorf("got different object back: %q != %q", dataToUpload, receivedContents)
	}
	fmt.Printf("**** got back \"%s\" ****\n", string(receivedContents))

	return nil
}

//JsConn is a javascript thing
type JsConn struct {
	ip   string
	port int
	id   int
}

var uint8Array = js.Global().Get("Uint8Array")

//NewJsConn returns a new JsConn
func NewJsConn(ip string, port int) (*JsConn, error) {
	creating := make(chan struct{})
	fmt.Println("connect start (Go)")
	var socketID, resultCode js.Value
	js.Global().Call("socketConnect", ip, port, js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		socketID = args[0]
		resultCode = args[1]
		fmt.Printf("connect close (Go) #%d\n", socketID.Int())
		close(creating)
		return nil
	}))
	<-creating
	fmt.Printf("connect (Go) socket id = %s\n", socketID.String())
	return &JsConn{ip: ip, port: port, id: socketID.Int()}, nil
}

func (c *JsConn) Read(b []byte) (n int, err error) {
	reading := make(chan struct{})
	fmt.Println("read start (Go)")
	var retVal, eof js.Value
	js.Global().Call("socketRead", c.id, len(b), js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		retVal = args[0]
		eof = args[1]
		fmt.Printf("read close (Go) #%d\n", c.id)
		close(reading)
		return nil
	}))
	<-reading
	fmt.Println("received read bytes (Go):")
	js.CopyBytesToGo(b, retVal)
	if eof.Bool() {
		fmt.Printf("EOF (Go) #%d\n", c.id)
		return retVal.Length(), io.EOF
	}
	return retVal.Length(), nil
}

func (c *JsConn) Write(b []byte) (n int, err error) {
	fmt.Println("write start (Go):")
	fmt.Println(len(b))
	buf := uint8Array.New(len(b))
	js.CopyBytesToJS(buf, b)

	writing := make(chan struct{})
	js.Global().Call("socketWrite", c.id, buf, js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		close(writing)
		return nil
	}))
	<-writing
	fmt.Println("write end (Go)")
	return len(b), nil
}
func (c *JsConn) Close() error {
	fmt.Println("closing socket (Go)")

	closing := make(chan struct{})
	js.Global().Call("socketDisconnect", c.id, js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		close(closing)
		return nil
	}))
	<-closing
	return nil
}
func (c *JsConn) LocalAddr() net.Addr {
	fmt.Println("local addr")
	return &addr{}
}
func (c *JsConn) RemoteAddr() net.Addr {
	fmt.Println("remote addr")
	return &addr{}
}
func (c *JsConn) SetDeadline(t time.Time) error {
	fmt.Println("set deadline")
	return nil
}
func (c *JsConn) SetReadDeadline(t time.Time) error {
	fmt.Println("set read deadline")
	return nil
}
func (c *JsConn) SetWriteDeadline(t time.Time) error {
	fmt.Println("set write deadline")
	return nil
}

type addr struct {
}

func (*addr) Network() string {
	return ""
}
func (*addr) String() string {
	return ""
}
