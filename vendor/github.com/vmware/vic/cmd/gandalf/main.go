// Copyright 2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
	"github.com/nlopes/slack"
)

const (
	GandalfContext     = "gandalf/thebot"
	PullApproveContext = "code-review/pullapprove"
	// ExistingContext is what we replace context with (we go from "gandalf/thebot", "code-review/pullapprove" to this)
	ExistingContext = `["code-review/pullapprove"]`

	GitHubStatusAPIURL   = "https://status.github.com/api/last-message.json"
	GitHubContextsAPIURL = "https://api.github.com/repos/%s/%s/branches/%s/protection/required_status_checks/contexts"

	DefaultUser   = "vmware"
	DefaultRepo   = "vic"
	DefaultBranch = "master"

	DefaultFellows = "mhagen,mwilliamson,ghicken"

	// 4 disabling merges to master automagically
	DroneFailureMessage = "finished with a failure status, find the logs"
)

var (
	config = GandalfConfig{}
)

type ActionType func(msg *slack.MessageEvent) string

type MultipleVar []string

func (i *MultipleVar) String() string {
	return fmt.Sprint(*i)
}

func (i *MultipleVar) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type GandalfConfig struct {
	GithubToken string
	SlackToken  string

	User   string
	Repo   string
	Branch string

	Fellows MultipleVar
}

type Gandalf struct {
	GandalfConfig

	slack  *slack.Client
	github *github.Client
	rtm    *slack.RTM

	// gandalf's prefix
	prefix string

	// name->id cache
	u2i map[string]string

	// text->fn
	actions map[string]ActionType
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())

	config.SlackToken = os.Getenv("SLACK_TOKEN")
	if config.SlackToken == "" {
		panic("missing slack token")
	}
	config.GithubToken = os.Getenv("GITHUB_TOKEN")
	if config.GithubToken == "" {
		panic("missing github token")
	}

	flag.StringVar(&config.User, "user", DefaultUser, "Name of the user")
	flag.StringVar(&config.Repo, "repo", DefaultRepo, "Name of the repo")
	flag.StringVar(&config.Branch, "branch", DefaultBranch, "Name of the branch")

	flag.Var(&config.Fellows, "fellow", "Fellow name")

	flag.Parse()

	// use the default if no fellow given
	if len(config.Fellows) == 0 {
		config.Fellows = strings.Split(DefaultFellows, ",")
	}
}

func (g *Gandalf) Setup() {
	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: g.GithubToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	g.github = github.NewClient(tc)
	g.slack = slack.New(g.SlackToken)

	g.rtm = g.slack.NewRTM()

	g.actions = map[string]ActionType{
		"status?": func(msg *slack.MessageEvent) string {
			ok, err := g.closed()
			if err != nil {
				return fmt.Sprintf("Something bad happened: %s", err)
			}
			if ok {
				return "Merges to master are closed..."
			}
			return "Merges to master are open..."
		},
		"disable merges to master": func(msg *slack.MessageEvent) string {
			if !g.fellow(msg.User) {
				return "You are not a fellow. You can't hold the ring."
			}
			if err := g.disable(); err != nil {
				return fmt.Sprintf("Something bad happened: %s", err)
			}
			return "One moment, I'll call the Github CEO to do that"
		},
		"enable merges to master": func(msg *slack.MessageEvent) string {
			if !g.fellow(msg.User) {
				return "You are not a fellow. You can't hold the ring."
			}
			if err := g.enable(); err != nil {
				return fmt.Sprintf("Something bad happened: %s", err)
			}
			return "Sure but it will cost you"
		},
		"who are you?": func(msg *slack.MessageEvent) string {
			return "I am Gandalf the Bot. You shall not merge..."
		},
		"what is your story?": func(msg *slack.MessageEvent) string {
			return "I used my last measure of strength to revert some commits. My spirit then left my body, having sacrificed myself to save the project. My spirit did not depart Github forever at this time. As the only one of the five Istari to stay true to his errand, I was sent back to mortal lands by Eru, and I became Gandalf the Bot"
		},
		"where are you?": func(msg *slack.MessageEvent) string {
			return "Hmmm, somewhere over the rainbow. I'm meeting with VCs to raise some capital for my new blockchain startup. Chains are the new rings."
		},
		"how are you doing?": func(msg *slack.MessageEvent) string {
			return "Ash nazg durbatulûk, ash nazg gimbatul,\nAsh nazg thrakatulûk agh burzum-ishi krimpatul."
		},
		"what do you think about scrum?": func(msg *slack.MessageEvent) string {
			return "One sprint to rule them all, one sprint to find them,\nOne sprint to bring them all and in the darkness bind them."
		},
		"how is github doing?": func(msg *slack.MessageEvent) string { return g.status() },
		"show me open prs":     func(msg *slack.MessageEvent) string { return g.pr() },
		"show me the fellows":  func(msg *slack.MessageEvent) string { return g.Fellows.String() },
	}

	// manage the connections
	go g.rtm.ManageConnection()

	// populate the cache
	users, err := g.slack.GetUsers()
	if err != nil {
		panic(fmt.Sprintf("failed to fetch users: %s", err))
	}

	for _, user := range users {
		g.u2i[user.Name] = user.ID
	}

	g.prefix = fmt.Sprintf("<@%s> ", g.u2i["gandalf"])
}

func main() {
	gandalf := Gandalf{
		GandalfConfig: config,
		u2i:           make(map[string]string),
		actions:       make(map[string]ActionType),
	}

	gandalf.Setup()

	// wait 4 events
	for msg := range gandalf.rtm.IncomingEvents {
		switch ev := msg.Data.(type) {
		case *slack.MessageEvent:
			gandalf.handle(ev)
		case *slack.RTMError:
			fmt.Printf("Error: %s\n", ev.Error())

		case *slack.InvalidAuthEvent:
			fmt.Printf("Invalid credentials")
			return

		default:
		}
	}
}

func (g *Gandalf) status() string {
	var status struct {
		Status  string    `json:"status"`
		Message string    `json:"body"`
		Created time.Time `json:"created_on"`
	}

	resp, err := http.Get(GitHubStatusAPIURL)
	if err != nil {
		return fmt.Sprintf("Err: %s", err)
	}
	defer resp.Body.Close()

	// #nosec: Errors unhandled.
	body, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &status)
	if err != nil {
		return fmt.Sprintf("Err: %s", err)
	}
	return fmt.Sprintf("Current Github Status : %s. Message : %s Reported at %s", status.Status, status.Message, status.Created)
}

func (g *Gandalf) fellow(id string) bool {
	// linear search bla bla
	for i := range g.Fellows {
		if id == g.u2i[g.Fellows[i]] {
			return true
		}
	}
	return false
}

func (g *Gandalf) pr() string {
	ctx := context.Background()

	opt := &github.PullRequestListOptions{Sort: "created", Direction: "desc"}

	prs, _, err := g.github.PullRequests.List(ctx, g.User, g.Repo, opt)
	if err != nil {
		return fmt.Sprintf("Err: %s", err)
	}

	response := ""
	for i := range prs {
		response = fmt.Sprintf("%s %d - %s (%s)\n", response, *prs[i].Number, *prs[i].Title, *prs[i].HTMLURL)
	}
	return response
}

func (g *Gandalf) closed() (bool, error) {
	ctx := context.Background()

	protection, _, err := g.github.Repositories.GetBranchProtection(ctx, g.User, g.Repo, g.Branch)
	if err != nil {
		return false, err
	}

	for i := range protection.RequiredStatusChecks.Contexts {
		if protection.RequiredStatusChecks.Contexts[i] == GandalfContext {
			return true, nil
		}
	}
	return false, nil
}

func (g *Gandalf) disable() error {
	ctx := context.Background()

	preq := &github.ProtectionRequest{
		RequiredStatusChecks: &github.RequiredStatusChecks{
			IncludeAdmins: false,
			Strict:        true,
			Contexts:      []string{GandalfContext, PullApproveContext},
		},
	}
	_, _, err := g.github.Repositories.UpdateBranchProtection(ctx, g.User, g.Repo, g.Branch, preq)
	if err != nil {
		return err
	}

	return nil
}

func (g *Gandalf) enable() error {
	// https://developer.github.com/v3/repos/branches/#replace-required-status-checks-contexts-of-protected-branch is not supported by the Go github API yet so we improvise
	url := fmt.Sprintf(GitHubContextsAPIURL, g.User, g.Repo, g.Branch)

	// #nosec: Errors unhandled.
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer([]byte(ExistingContext)))
	req.Header.Set("Authorization", fmt.Sprintf("token %s", g.GithubToken))
	req.Header.Set("Accept", "application/vnd.github.loki-preview+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%#v", resp)
	}
	return nil
}

func (g *Gandalf) handle(msg *slack.MessageEvent) {
	// is it drone failure?
	if strings.Contains(msg.Text, DroneFailureMessage) {
		if err := g.disable(); err != nil {
			g.rtm.SendMessage(g.rtm.NewOutgoingMessage(fmt.Sprintf("Something bad happened: %s", err), msg.Channel))
		}
		g.rtm.SendMessage(g.rtm.NewOutgoingMessage("They shall not merge! Closing master to merges...", msg.Channel))
		return
	}

	// @gandalf TEXT
	if strings.HasPrefix(msg.Text, g.prefix) {
		text := msg.Text
		text = strings.TrimPrefix(text, g.prefix)
		text = strings.TrimSpace(text)
		text = strings.ToLower(text)

		if response, ok := g.actions[text]; ok {
			g.rtm.SendMessage(g.rtm.NewOutgoingMessage(response(msg), msg.Channel))
			return
		}

		cmds := make([]string, 0, len(g.actions))
		for key := range g.actions {
			cmds = append(cmds, key)
		}
		g.rtm.SendMessage(g.rtm.NewOutgoingMessage(
			fmt.Sprintf("Sorry, I can't help with that yet. I can understand following;\n```%s```\n", strings.Join(cmds, "\n")),
			msg.Channel,
		))
	}
}
