package client

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/hyperhq/hyper-api/types/events"
	signutil "github.com/hyperhq/websocket-client/go/util"
	"golang.org/x/net/context"
)

// Events returns a stream of events in the daemon. It's up to the caller to close the stream
// by cancelling the context. Once the stream has been completely read an io.EOF error will
// be sent over the error channel. If an error is sent all processing will be stopped. It's up
// to the caller to reopen the stream in the event of an error by reinvoking this method.
func (cli *DockerCli) Events(ctx context.Context) (<-chan events.Message, <-chan error) {
	messages := make(chan events.Message)
	errs := make(chan error, 1)

	go func() {
		hostUrl, err := url.Parse(cli.host)
		if err != nil {
			errs <- err
			return
		}

		cloudConfig, existed := cli.configFile.CloudConfig[cli.host]
		if !existed {
			errs <- errors.New("Please specify 'accessKey' and 'secretKey'!")
			return
		}

		// TODO: add filter when we have other type event.
		var u = url.URL{Scheme: "wss", Host: hostUrl.Host, Path: "/events/ws"}

		// add sign to header
		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			errs <- err
			return
		}

		req.URL = &u
		req = signutil.Sign4(cloudConfig.AccessKey, cloudConfig.SecretKey, req)

		// connect to websocket server
		config := &tls.Config{
			InsecureSkipVerify: true,
		}
		dialer := websocket.Dialer{
			TLSClientConfig: config,
		}

		ws, resp, err := dialer.Dial(u.String(), req.Header)
		if err != nil {
			errs <- err
			return
		}
		defer ws.Close()

		if resp.ContentLength > 0 {
			defer resp.Body.Close()
			ioutil.ReadAll(resp.Body)
		}

		// process websocket message
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				errs <- err
				return
			}

			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			default:
				var event events.Message
				err := json.Unmarshal([]byte(message), &event)
				if err != nil {
					errs <- err
					return
				}

				select {
				case messages <- event:
				case <-ctx.Done():
					errs <- ctx.Err()
					return
				}
			}
		}
	}()

	return messages, errs
}
