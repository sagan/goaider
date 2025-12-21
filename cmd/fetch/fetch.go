package fetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
	"github.com/sagan/goaider/util/stringutil"
)

const MAX_TRIES = -1

// standard udp public DNS servers
var DnsServers = []string{
	"1.1.1.1:53",
	"8.8.8.8:53",
	"9.9.9.9:53",
	"2606:4700:4700::1111",
	"2001:4860:4860::8888",
}

var fetchCmd = &cobra.Command{
	Use:   "fetch {url} [{additional_url}...]",
	Short: "Fetch a url by all means",
	Long: `Fetch a url by all means.

It tries to fetch the url via all available network interfaces.

You can provide multiple url. If any url returns a success response from any interface,
the program will output the response and exit 0.

It outputs to stdout by default.
`,
	RunE: doFetch,
	Args: cobra.MinimumNArgs(1),
}

var (
	flagUpdate   bool
	flagForce    bool
	flagText     bool
	flagMaxTries int
	flagOutput   string
	flagExec     string // execute a cmdline on success
)

func init() {
	cmd.RootCmd.AddCommand(fetchCmd)
	fetchCmd.Flags().BoolVarP(&flagText, "text", "", false, "Treat http response body as text, "+
		`normalize it and output canonical UTF-8 (without BOM) and \n line break text contents`)
	fetchCmd.Flags().BoolVarP(&flagUpdate, "update", "", false,
		"Update file mode. Use resonse body to update existing file only if their contents are not the same. "+
			`Implies --force`)
	fetchCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force mode. Overwrite existing file")
	fetchCmd.Flags().IntVarP(&flagMaxTries, "max-tries", "", MAX_TRIES, "Max tries for each interface. -1 for infinite")
	fetchCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output path. Use "-" for stdout`)
	fetchCmd.Flags().StringVarP(&flagExec, "exec", "x", "",
		`Execute a cmdline (using system shell) on success. The response body is passed to cmdline as stdin. `+
			`If --update flag is set, the cmdline is executed only if local file is updated`)
}

type Request struct {
	Url      *url.URL
	MaxTries int
	Ifname   string
	Text     bool // text mode
}

type Response struct {
	Data    []byte
	Done    bool
	Err     error
	Request *Request
}

func doFetch(cmd *cobra.Command, args []string) error {
	if flagOutput != "" && flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce && !flagUpdate) {
			return fmt.Errorf("output file %q exists or access failed. err: %w", flagOutput, err)
		}
	}

	var urlObjs []*url.URL
	for _, argUrl := range args {
		urlObj, err := url.Parse(argUrl)
		if err != nil {
			return fmt.Errorf("invalid url %q: %w", argUrl, err)
		}
		urlObjs = append(urlObjs, urlObj)
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	if len(ifaces) == 0 {
		return fmt.Errorf("no inferface found")
	}
	log.Printf("Use %d interfaces %s to fetch %d urls %q", len(ifaces), util.ToJson(ifaces), len(args), args)

	ch := make(chan *Response)
	cnt := 0
	for _, iface := range ifaces {
		for _, urlObj := range urlObjs {
			cnt++
			go run(ch, &Request{
				Url:      urlObj,
				Ifname:   iface.Name,
				MaxTries: flagMaxTries,
				Text:     flagText,
			})
		}
	}

	for {
		resp := <-ch
		if resp.Err == nil {
			log.Printf("Success: interface %q got result %d bytes", resp.Request.Ifname, len(resp.Data))
			if flagOutput == "-" {
				_, err = io.Copy(cmd.OutOrStdout(), bytes.NewReader(resp.Data))
			} else {
				if flagUpdate {
					exists, err := util.FileExists(flagOutput)
					if err != nil {
						return err
					}
					if exists {
						existingHash, err := util.HashFile(flagOutput, constants.HASH_SHA256, true)
						if err != nil {
							return err
						}
						hash, err := util.Hash(bytes.NewReader(resp.Data), constants.HASH_SHA256, true)
						if err != nil {
							panic(err)
						}
						if existingHash == hash {
							log.Printf("same contents as existing file, no update")
							return nil
						}
					}
				}
				err = atomic.WriteFile(flagOutput, bytes.NewReader((resp.Data)))
			}
			if flagExec != "" {
				err := helper.RunCmdline(flagExec, true, bytes.NewReader(resp.Data), nil, nil)
				log.Printf("Exec %v, err=%v", flagExec, err)
			}
			return err
		}
		log.Printf("interface %q failed to fetch url: %v, done: %t", resp.Request.Ifname, resp.Err, resp.Done)
		if resp.Done {
			cnt--
		}
		if cnt == 0 {
			return fmt.Errorf("all interfaces failed")
		}
	}
}

func run(ch chan<- *Response, req *Request) {
	log.Printf("Use interface %q for %q", req.Ifname, req.Url.String())
	iface, err := net.InterfaceByName(req.Ifname)
	if err != nil {
		ch <- &Response{Err: err, Request: req, Done: true}
		return
	}
	addrs, err := iface.Addrs()
	if err != nil || len(addrs) == 0 {
		ch <- &Response{Err: fmt.Errorf("no address found"), Request: req, Done: true}
		return
	}

	addr := req.Url.Host
	if req.Url.Port() == "" {
		switch req.Url.Scheme {
		case "http":
			addr += ":80"
		case "https":
			addr += ":443"
		default:
			ch <- &Response{Err: fmt.Errorf("unsupported scheme: %s", req.Url.Scheme), Request: req, Done: true}
			return
		}
	}

	control := getControl(req.Ifname)

	tries := -1
	for {
		tries++
		if req.MaxTries > 0 && tries >= req.MaxTries {
			break
		}
		if tries > 0 {
			time.Sleep(util.CalculateBackoff(time.Second, time.Second*60, tries))
		}

		localAddr := addrs[0].(*net.IPNet).IP.String()
		dialer := &net.Dialer{
			Resolver: &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{
						Timeout: 10 * time.Second,
						Control: control,
					}
					// don't use system dns (network & address).
					i := tries % (len(DnsServers))
					return d.DialContext(ctx, "udp", DnsServers[i])
				},
			},
			LocalAddr: &net.TCPAddr{
				IP:   net.ParseIP(localAddr), // The specific source IP to bind to
				Port: 0,                      // Let the OS pick an ephemeral port
			},
			Timeout: 30 * time.Second,
			Control: control,
		}

		customTransport := &http.Transport{
			DialContext:         dialer.DialContext,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			MaxIdleConnsPerHost: 2, // Default limit per host
		}

		client := &http.Client{
			Transport: customTransport,
			Timeout:   30 * time.Second,
		}

		var resp *http.Response
		// Use the client as usual
		resp, err = client.Get(req.Url.String())
		if err != nil {
			ch <- &Response{Err: err, Request: req}
			continue
		}

		if resp.StatusCode != 200 {
			if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
				log.Warnf("interface %q got http status %d, retrying", req.Ifname, resp.StatusCode)
				resp.Body.Close()
				continue
			} else {
				ch <- &Response{Err: fmt.Errorf("http status %d", resp.StatusCode), Request: req, Done: true}
				resp.Body.Close()
				return
			}
		}

		var body io.Reader = resp.Body
		if req.Text {
			body = stringutil.GetTextReader(body)
		}
		var data []byte
		data, err = io.ReadAll(body)
		resp.Body.Close()
		if err != nil {
			ch <- &Response{Err: err, Request: req}
			continue
		}
		ch <- &Response{Data: data, Request: req, Done: true}
		return
	}
	ch <- &Response{Err: fmt.Errorf("too many failures, last error: %w", err), Done: true}
}
