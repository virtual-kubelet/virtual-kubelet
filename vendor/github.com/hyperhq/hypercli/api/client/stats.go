package client

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/events"
	Cli "github.com/hyperhq/hypercli/cli"
	"github.com/hyperhq/hypercli/cli/command"
	"github.com/hyperhq/hypercli/cli/command/formatter"
	"golang.org/x/net/context"
)

// CmdStats displays a live stream of resource usage statistics for one or more containers.
//
// This shows real-time information on CPU usage, memory usage, and network I/O.
//
// Usage: hyper stats [OPTIONS] [CONTAINER...]
func (cli *DockerCli) CmdStats(args ...string) error {
	cmd := Cli.Subcmd("stats", []string{"[CONTAINER...]"}, Cli.DockerCommands["stats"].Description, true)
	all := cmd.Bool([]string{"a", "-all"}, false, "Show all containers (default shows just running)")
	noStream := cmd.Bool([]string{"-no-stream"}, false, "Disable streaming stats and only pull the first result")

	cmd.ParseFlags(args, true)

	names := cmd.Args()
	showAll := len(names) == 0
	closeChan := make(chan error)
	ctx := context.Background()
	eventChan := make(chan events.Message)

	// monitorContainerEvents watches for container creation and removal (only
	// used when calling `docker stats` without arguments).
	monitorContainerEvents := func(started chan<- struct{}, c chan events.Message) {
		eventq, errq := cli.Events(ctx)

		// Whether we successfully subscribed to eventq or not, we can now
		// unblock the main goroutine.
		close(started)

		for {
			select {
			case event := <-eventq:
				c <- event
			case err := <-errq:
				closeChan <- err
				return
			}
		}
	}

	// waitFirst is a WaitGroup to wait first stat data's reach for each container
	waitFirst := &sync.WaitGroup{}

	cStats := stats{}
	// getContainerList simulates creation event for all previously existing
	// containers (only used when calling `docker stats` without arguments).
	getContainerList := func() {
		options := types.ContainerListOptions{
			All: *all,
		}
		cs, err := cli.client.ContainerList(ctx, options)
		if err != nil {
			closeChan <- err
		}
		for _, container := range cs {
			s := formatter.NewContainerStats(container.ID[:12])
			if cStats.add(s) {
				waitFirst.Add(1)
				go collect(ctx, s, cli, !*noStream, waitFirst, eventChan, false)
			}
		}
	}

	if showAll {
		// If no names were specified, start a long running goroutine which
		// monitors container events. We make sure we're subscribed before
		// retrieving the list of running containers to avoid a race where we
		// would "miss" a creation.
		started := make(chan struct{})
		eh := command.InitEventHandler()

		eh.Handle("start", func(e events.Message) {
			s := formatter.NewContainerStats(e.ID[:12])
			if cStats.add(s) {
				waitFirst.Add(1)
				go collect(ctx, s, cli, !*noStream, waitFirst, eventChan, false)
			}
		})

		eh.Handle("stop", func(e events.Message) {
			if !*all {
				cStats.remove(e.ID[:12])
			}
		})

		go eh.Watch(eventChan)
		go monitorContainerEvents(started, eventChan)
		defer close(eventChan)
		<-started

		// Start a short-lived goroutine to retrieve the initial list of
		// containers.
		getContainerList()
	} else {
		// Artificially send creation events for the containers we were asked to
		// monitor (same code path than we use when monitoring all containers).
		for _, name := range names {
			s := formatter.NewContainerStats(name)
			if cStats.add(s) {
				waitFirst.Add(1)
				go collect(ctx, s, cli, !*noStream, waitFirst, nil, true)
			}
		}

		// We don't expect any asynchronous errors: closeChan can be closed.
		close(closeChan)

		// Do a quick pause to detect any error with the provided list of
		// container names.
		time.Sleep(1500 * time.Millisecond)
		var errs []string
		cStats.mu.Lock()
		for _, c := range cStats.cs {
			cErr := c.GetError()
			if cErr != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", c.Name, cErr))
			}
		}
		cStats.mu.Unlock()
		if len(errs) > 0 {
			return fmt.Errorf("%s", strings.Join(errs, ", "))
		}
	}

	// before print to screen, make sure each container get at least one valid stat data
	waitFirst.Wait()

	statsCtx := formatter.Context{
		Output: cli.out,
		Format: formatter.NewStatsFormat(formatter.TableFormatKey),
	}

	cleanScreen := func() {
		if !*noStream {
			fmt.Fprint(cli.out, "\033[2J")
			fmt.Fprint(cli.out, "\033[H")
		}
	}

	var err error
	for range time.Tick(500 * time.Millisecond) {
		cleanScreen()
		ccstats := []formatter.StatsEntry{}
		cStats.mu.Lock()
		for _, c := range cStats.cs {
			ccstats = append(ccstats, c.GetStatistics())
		}
		cStats.mu.Unlock()
		if err = formatter.ContainerStatsWrite(statsCtx, ccstats); err != nil {
			break
		}
		if len(cStats.cs) == 0 && !showAll {
			break
		}
		if *noStream {
			break
		}
		select {
		case err, ok := <-closeChan:
			if ok {
				if err != nil {
					// this is suppressing "unexpected EOF" in the cli when the
					// daemon restarts so it shutdowns cleanly
					if err == io.ErrUnexpectedEOF {
						return nil
					}
					return err
				}
			}
		default:
			// just skip
		}
	}
	return err
}
