package fetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"time"

	"github.com/google/shlex"
	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
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
	flagForce    bool
	flagMaxTries int
	flagOutput   string
	flagExec     string // execute a cmdline on success
)

func init() {
	cmd.RootCmd.AddCommand(fetchCmd)
	fetchCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting existing file")
	fetchCmd.Flags().IntVarP(&flagMaxTries, "max-tries", "", MAX_TRIES, "Max tries for each interface. -1 for infinite")
	fetchCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output path. Use "-" for stdout`)
	fetchCmd.Flags().StringVarP(&flagExec, "exec", "x", "",
		`Execute a cmdline on success. The response body is passed to cmdline as stdin`)
}

type Request struct {
	Url      *url.URL
	MaxTries int
	Ifname   string
}

type Response struct {
	Data    []byte
	Done    bool
	Err     error
	Request *Request
}

func doFetch(cmd *cobra.Command, args []string) error {
	if flagOutput != "" && flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
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
				err = atomic.WriteFile(flagOutput, bytes.NewReader((resp.Data)))
			}
			if flagExec != "" {
				if args, err := shlex.Split(flagExec); err == nil && len(args) > 0 {
					command := exec.Command(args[0], args[1:]...)
					command.Stdin = bytes.NewReader(resp.Data)
					err = command.Run()
					log.Printf("Exec %v, err=%v", args, err)
				} else {
					log.Errorf("Invalid exec: err=%v", err)
				}
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
					}
					i := tries % (len(DnsServers) + 1)
					if i == 0 {
						return d.DialContext(ctx, network, address)
					}
					return d.DialContext(ctx, "udp", DnsServers[i-1])
				},
			},
			LocalAddr: &net.TCPAddr{
				IP:   net.ParseIP(localAddr), // The specific source IP to bind to
				Port: 0,                      // Let the OS pick an ephemeral port
			},
			Timeout: 30 * time.Second,
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

		var data []byte
		data, err = io.ReadAll(resp.Body)
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
