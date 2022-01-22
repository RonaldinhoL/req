package req

import (
	"encoding/json"
	"golang.org/x/net/publicsuffix"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"
)

func DefaultClient() *Client {
	return defaultClient
}

func SetDefaultClient(c *Client) {
	if c != nil {
		defaultClient = c
	}
}

var defaultClient *Client = C()

type Client struct {
	log          Logger
	t            *Transport
	t2           *http2Transport
	dumpOptions  *DumpOptions
	httpClient   *http.Client
	jsonDecoder  *json.Decoder
	commonHeader map[string]string
}

func copyCommonHeader(h map[string]string) map[string]string {
	if h == nil {
		return nil
	}
	m := make(map[string]string)
	for k, v := range h {
		m[k] = v
	}
	return m
}

func (c *Client) R() *Request {
	req := &http.Request{
		Header:     make(http.Header),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	return &Request{
		client:      c,
		httpRequest: req,
	}
}

func (c *Client) AutoDiscardResponseBody() *Client {
	return c.ResponseOptions(DiscardResponseBody())
}

// TestMode is like DebugMode, but discard response body, so you can
// dump responses without read response body
func (c *Client) TestMode() *Client {
	return c.DebugMode().AutoDiscardResponseBody()
}

const (
	userAgentFirefox = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:95.0) Gecko/20100101 Firefox/95.0"
	userAgentChrome  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.71 Safari/537.36"
)

// DebugMode enables dump for requests and responses, and set user
// agent to pretend to be a web browser, Avoid returning abnormal
// data from some sites.
func (c *Client) DebugMode() *Client {
	return c.AutoDecodeTextContent().
		Dump(true).
		Logger(NewLogger(os.Stdout)).
		UserAgent(userAgentChrome)
}

func (c *Client) Logger(log Logger) *Client {
	if log == nil {
		return c
	}
	c.log = log
	return c
}

func (c *Client) ResponseOptions(opts ...ResponseOption) *Client {
	for _, opt := range opts {
		opt(&c.t.ResponseOptions)
	}
	return c
}

func (c *Client) Timeout(d time.Duration) *Client {
	c.httpClient.Timeout = d
	return c
}

func (c *Client) getDumpOptions() *DumpOptions {
	if c.dumpOptions == nil {
		c.dumpOptions = newDefaultDumpOptions()
	}
	return c.dumpOptions
}

func (c *Client) enableDump() {
	if c.t.dump != nil { // dump already started
		return
	}
	c.t.EnableDump(c.getDumpOptions())
}

// DumpToFile indicates that the content should dump to the specified filename.
func (c *Client) DumpToFile(filename string) *Client {
	file, err := os.Create(filename)
	if err != nil {
		logf(c.log, "create dump file error: %v", err)
		return c
	}
	c.getDumpOptions().Output = file
	return c
}

// DumpTo indicates that the content should dump to the specified destination.
func (c *Client) DumpTo(output io.Writer) *Client {
	c.getDumpOptions().Output = output
	c.enableDump()
	return c
}

// DumpAsync indicates that the dump should be done asynchronously,
// can be used for debugging in production environment without
// affecting performance.
func (c *Client) DumpAsync() *Client {
	o := c.getDumpOptions()
	o.Async = true
	c.enableDump()
	return c
}

// DumpOnlyResponse indicates that should dump the responses' head and response.
func (c *Client) DumpOnlyResponse() *Client {
	o := c.getDumpOptions()
	o.ResponseHead = true
	o.ResponseBody = true
	o.RequestBody = false
	o.RequestHead = false
	c.enableDump()
	return c
}

// DumpOnlyRequest indicates that should dump the requests' head and response.
func (c *Client) DumpOnlyRequest() *Client {
	o := c.getDumpOptions()
	o.RequestHead = true
	o.RequestBody = true
	o.ResponseBody = false
	o.ResponseHead = false
	c.enableDump()
	return c
}

// DumpOnlyBody indicates that should dump the body of requests and responses.
func (c *Client) DumpOnlyBody() *Client {
	o := c.getDumpOptions()
	o.RequestBody = true
	o.ResponseBody = true
	o.RequestHead = false
	o.ResponseHead = false
	c.enableDump()
	return c
}

// DumpOnlyHead indicates that should dump the head of requests and responses.
func (c *Client) DumpOnlyHead() *Client {
	o := c.getDumpOptions()
	o.RequestHead = true
	o.ResponseHead = true
	o.RequestBody = false
	o.ResponseBody = false
	c.enableDump()
	return c
}

// DumpAll indicates that should dump both requests and responses' head and body.
func (c *Client) DumpAll() *Client {
	o := c.getDumpOptions()
	o.RequestHead = true
	o.RequestBody = true
	o.ResponseHead = true
	o.ResponseBody = true
	c.enableDump()
	return c
}

// NewRequest is the alias of R()
func (c *Client) NewRequest() *Request {
	return c.R()
}

func (c *Client) AutoDecodeTextContent() *Client {
	return c.ResponseOptions(AutoDecodeTextContent())
}

func (c *Client) UserAgent(userAgent string) *Client {
	return c.CommonHeader("User-Agent", userAgent)
}

func (c *Client) CommonHeader(key, value string) *Client {
	if c.commonHeader == nil {
		c.commonHeader = make(map[string]string)
	}
	c.commonHeader[key] = value
	return c
}

// Dump if true, enables dump requests and responses,  allowing you
// to clearly see the content of all requests and responses，which
// is very convenient for debugging APIs.
// Dump if false, disable the dump behaviour.
func (c *Client) Dump(enable bool) *Client {
	if !enable {
		c.t.DisableDump()
		return c
	}
	c.enableDump()
	return c
}

// DumpOptions configures the underlying Transport's DumpOptions
func (c *Client) DumpOptions(opt *DumpOptions) *Client {
	if opt == nil {
		return c
	}
	c.dumpOptions = opt
	if c.t.dump != nil {
		c.t.dump.DumpOptions = opt
	}
	return c
}

// EnableDump enables dump requests and responses,  allowing you
// to clearly see the content of all requests and responses，which
// is very convenient for debugging APIs.
// EnableDump accepet options for custom the dump behavior, such
// as DumpAsync, DumpHead, DumpBody, DumpRequest, DumpResponse,
// DumpAll, DumpTo, DumpToFile
//func (c *Client) EnableDump(opts ...DumpOption) *Client {
//	if len(opts) > 0 {
//		if c.dumpOptions == nil {
//			c.dumpOptions = &DumpOptions{}
//		}
//		c.dumpOptions.set(opts...)
//	} else if c.dumpOptions == nil {
//		c.dumpOptions = defaultDumpOptions.Clone()
//	}
//	c.t.EnableDump(c.dumpOptions)
//	return c
//}

// NewClient is the alias of C()
func NewClient() *Client {
	return C()
}

func (c *Client) Clone() *Client {
	t := c.t.Clone()
	t2, _ := http2ConfigureTransports(t)
	cc := *c.httpClient
	cc.Transport = t
	return &Client{
		httpClient:   &cc,
		t:            t,
		t2:           t2,
		dumpOptions:  c.dumpOptions.Clone(),
		jsonDecoder:  c.jsonDecoder,
		commonHeader: copyCommonHeader(c.commonHeader),
	}
}

func C() *Client {
	t := &Transport{
		ForceAttemptHTTP2:     true,
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	t2, _ := http2ConfigureTransports(t)
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	httpClient := &http.Client{
		Transport: t,
		Jar:       jar,
		Timeout:   2 * time.Minute,
	}
	c := &Client{
		log:        &emptyLogger{},
		httpClient: httpClient,
		t:          t,
		t2:         t2,
	}
	return c
}
