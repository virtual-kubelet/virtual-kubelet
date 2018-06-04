// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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
	"archive/zip"
	"compress/gzip"
	"crypto/tls"
	"crypto/x509"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"context"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/google/uuid"
	gorillacontext "github.com/gorilla/context"

	"github.com/vmware/vic/lib/vicadmin"
	"github.com/vmware/vic/pkg/filelock"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
)

type server struct {
	l    net.Listener
	addr string
	mux  *http.ServeMux
	uss  *UserSessionStore
}

// LoginPageData contains items needed to render the login page template
type LoginPageData struct {
	Hostname     string
	SystemTime   string
	InvalidLogin string
}

type format int

const (
	formatTGZ format = iota
	formatZip
)

var beginningOfTime = time.Unix(0, 0).Format(time.RFC3339)

const (
	sessionExpiration      = time.Hour * 24
	sessionCookieKey       = "session-data"
	sessionCreationTimeKey = "created"
	sessionKey             = "session-id"
	ipAddressKey           = "ip"
	loginPagePath          = "/authentication"
	authFailure            = loginPagePath + "?unauthorized"
	genericErrorMessage    = "Internal Server Error; see /var/log/vic/vicadmin.log for details" // for http errors that shouldn't be displayed in the browser to the user
)

func (s *server) listen() error {
	defer trace.End(trace.Begin(""))

	var err error
	var certificate *tls.Certificate
	s.uss = NewUserSessionStore()
	if vchConfig.HostCertificate != nil {
		certificate, err = vchConfig.HostCertificate.Certificate()
	} else {
		var c tls.Certificate
		if c, err = tls.X509KeyPair(
			rootConfig.serverCert.Cert.Bytes(),
			rootConfig.serverCert.Key.Bytes()); err != nil {
			log.Errorf("Could not generate self-signed certificate for vicadmin running with due to error %s", err.Error())
			return err
		}
		certificate = &c
	}
	if err != nil {
		log.Errorf("Could not load certificate from config - running without TLS: %s", err)

		s.l, err = net.Listen("tcp", s.addr)
		return err
	}

	// FIXME: assignment copies lock value to tlsConfig: crypto/tls.Config contains sync.Once contains sync.Mutex
	tlsconfig := func(c *tls.Config) *tls.Config {
		// if there are CAs, then TLS is enabled
		if len(vchConfig.CertificateAuthorities) != 0 {
			if c.ClientCAs == nil {
				c.ClientCAs = x509.NewCertPool()
			}
			if !c.ClientCAs.AppendCertsFromPEM(vchConfig.CertificateAuthorities) {
				log.Errorf("Unable to load CAs from config; client auth via certificate will not function")
			}
			c.ClientAuth = tls.VerifyClientCertIfGiven
		} else {
			log.Warnf("No certificate authorities found for certificate-based authentication. This may be intentional, however, certificate-based authentication is disabled")
		}

		// #nosec: TLS InsecureSkipVerify may be true
		return &tls.Config{
			Certificates:             c.Certificates,
			NameToCertificate:        c.NameToCertificate,
			GetCertificate:           c.GetCertificate,
			RootCAs:                  c.RootCAs,
			NextProtos:               c.NextProtos,
			ServerName:               c.ServerName,
			ClientAuth:               c.ClientAuth,
			ClientCAs:                c.ClientCAs,
			InsecureSkipVerify:       c.InsecureSkipVerify,
			CipherSuites:             c.CipherSuites,
			PreferServerCipherSuites: c.PreferServerCipherSuites,
			SessionTicketsDisabled:   c.SessionTicketsDisabled,
			SessionTicketKey:         c.SessionTicketKey,
			ClientSessionCache:       c.ClientSessionCache,
			MinVersion:               tls.VersionTLS12,
			MaxVersion:               c.MaxVersion,
			CurvePreferences:         c.CurvePreferences,
		}
	}(tlsconfig.ServerDefault())

	tlsconfig.Certificates = []tls.Certificate{*certificate}

	innerListener, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Fatal(err)
		return err
	}

	s.l = tls.NewListener(innerListener, tlsconfig)
	return nil
}

func (s *server) listenPort() int {
	return s.l.Addr().(*net.TCPAddr).Port
}

// Enforces authentication on route `link` and runs `handler` on successful auth
func (s *server) AuthenticatedHandle(link string, h http.Handler) {
	s.Authenticated(link, h.ServeHTTP)
}

func (s *server) Handle(link string, h http.Handler) {
	log.Debugf("%s --- %s", time.Now().String(), link)
	s.mux.Handle(link, gorillacontext.ClearHandler(h))
}

// Enforces authentication on route `link` and runs `handler` on successful auth
func (s *server) Authenticated(link string, handler func(http.ResponseWriter, *http.Request)) {
	defer trace.End(trace.Begin(""))

	authHandler := func(w http.ResponseWriter, r *http.Request) {
		// #nosec: Errors unhandled because it is okay if the cookie doesn't exist.
		websession, _ := s.uss.cookies.Get(r, sessionCookieKey)

		if len(r.TLS.PeerCertificates) > 0 {
			// the user is authenticated by certificate at connection time
			log.Infof("Authenticated connection via client certificate with serial %s from %s", r.TLS.PeerCertificates[0].SerialNumber, r.RemoteAddr)
			key := uuid.New().String()

			vs, err := vSphereSessionGet(&rootConfig.Config)
			if err != nil {
				log.Errorf("Unable to get vSphere session with default config for cert-auth'd user")
				http.Error(w, genericErrorMessage, http.StatusInternalServerError)
				return
			}
			usersess := s.uss.Add(key, &rootConfig.Config, vs)

			timeNow, err := usersess.created.MarshalText()
			if err != nil {
				// it's probably safe to ignore this error since we just created usersess.created when we called Add() above
				// but just in case..
				log.Errorf("Failed to unmarshal time object %+v into text due to error: %s", usersess.created, err)
				http.Error(w, genericErrorMessage, http.StatusInternalServerError)
				return
			}

			websession.Values[sessionCreationTimeKey] = string(timeNow)
			websession.Values[sessionKey] = key

			remoteAddr := strings.SplitN(r.RemoteAddr, ":", 2)
			if len(remoteAddr) != 2 { // TODO: ctrl+f RemoteAddr and move this routine to helper
				log.Errorf("Format of IP address %s (should be IP:PORT) not recognized", r.RemoteAddr)
				http.Error(w, genericErrorMessage, http.StatusInternalServerError)
				return
			}
			websession.Values[ipAddressKey] = remoteAddr[0]
			err = websession.Save(r, w)
			if err != nil {
				log.Errorf("Could not create session for user authenticated via client certificate due to error \"%s\"", err.Error())
				http.Error(w, genericErrorMessage, http.StatusInternalServerError)
				return
			}

			// user was authenticated via cert
			handler(w, r)
			return
		}

		c := websession.Values[sessionCreationTimeKey]
		if c == nil { // no cookie, so redirect to login
			log.Infof("No authentication token: %+v", websession.Values)
			http.Redirect(w, r, loginPagePath, http.StatusSeeOther)
			return
		}

		// here we have a cookie, but we need to make sure it's not expired:
		// parse the cookie creation time
		created, err := time.Parse(time.RFC3339, c.(string))
		if err != nil {
			// we pulled this value out of a cookie, so if it doesn't parse, it might've been tampered with
			// though the cookie's encrypted so that would destroy the whole cookie..
			// Handling the error in any case:
			log.Errorf("Couldn't parse time out of retrieved cookie due to error %s", err.Error())
			http.Error(w, genericErrorMessage, http.StatusInternalServerError)
			return
		}

		// cookie exists but is expired
		if time.Since(created) > sessionExpiration {
			http.Redirect(w, r, loginPagePath, http.StatusTemporaryRedirect)
			return
		}

		// verify that the auth token is being used by the same IP it was created for
		c = websession.Values[ipAddressKey]
		if c == nil {
			log.Errorf("Couldn't get IP address out of cookie for user connecting from %s at %s", r.RemoteAddr, time.Now())
			http.Redirect(w, r, loginPagePath, http.StatusTemporaryRedirect)
			return
		}

		connectingAddr := strings.SplitN(r.RemoteAddr, ":", 2)
		if len(connectingAddr) != 2 { // TODO: ctrl+f r.RemoteAddr and move this routine to helper
			log.Errorf("Format of IP address %s (should be IP:PORT) not recognized", r.RemoteAddr)
			http.Error(w, genericErrorMessage, http.StatusInternalServerError)
			return
		}
		if c.(string) != connectingAddr[0] {
			log.Warnf("User with a valid auth cookie from %s has reappeared at %s. Their token will be expired.", c.(string), connectingAddr[0])
			s.logoutHandler(w, r)
			return
		}

		// if the date & remote IP on the cookie were valid, then the user is authenticated
		log.Infof("User with a valid auth cookie at %s is authenticated.", connectingAddr[0])
		handler(w, r)
	}
	s.Handle(link, http.HandlerFunc(authHandler))
}

// renders the page for login and handles authorization requests
func (s *server) loginPage(res http.ResponseWriter, req *http.Request) {
	defer trace.End(trace.Begin(""))
	var invalidLoginMessage = ""

	if req.Method == "POST" {
		// take the form data and use it to try to authenticate with vsphere

		// create a userconfig
		userconfig := session.Config{
			Insecure:       false,
			Thumbprint:     rootConfig.Thumbprint,
			Keepalive:      rootConfig.Keepalive,
			ClusterPath:    rootConfig.ClusterPath,
			DatacenterPath: rootConfig.DatacenterPath,
			DatastorePath:  rootConfig.DatastorePath,
			HostPath:       rootConfig.Config.HostPath,
			PoolPath:       rootConfig.PoolPath,
			User:           url.UserPassword(req.FormValue("username"), req.FormValue("password")),
			Service:        rootConfig.Service,
		}

		// check login
		vs, err := vSphereSessionGet(&userconfig)
		if err != nil || vs == nil {
			// something went wrong or we could not authenticate
			log.Warnf("%s failed to authenticate ", req.RemoteAddr)
			invalidLoginMessage = "Authentication failed due to incorrect credential(s)"
			hostName, err := os.Hostname()
			if err != nil {
				hostName = "VCH"
			}
			loginPageData := &LoginPageData{
				Hostname:     hostName,
				SystemTime:   time.Now().Format(time.UnixDate),
				InvalidLogin: invalidLoginMessage,
			}

			tmpl, err := template.ParseFiles("auth.html")
			err = tmpl.ExecuteTemplate(res, "auth.html", loginPageData)
			if err != nil {
				log.Errorf("Error parsing template: %s", err)
				http.Error(res, genericErrorMessage, http.StatusInternalServerError)
				return
			}
			return
		}

		// successful login above; user is authenticated, reported for audit purposes
		log.Debugf("User %s from %s was successfully authenticated", userconfig.User.Username(), req.RemoteAddr)

		// create a token to save as an encrypted & signed cookie
		websession, err := s.uss.cookies.Get(req, sessionCookieKey)
		if websession == nil {
			log.Errorf("Web session object could not be created due to error %s", err)
			http.Error(res, genericErrorMessage, http.StatusInternalServerError)
			return
		}

		key := uuid.New().String()
		userconfig.User = nil
		userconfig.Service = ""
		us := s.uss.Add(key, &userconfig, vs)

		timeNow, err := us.created.MarshalText()
		if err != nil {
			log.Errorf("Failed to unmarshal time object %+v into text due to error: %s", us.created, err)
			http.Error(res, genericErrorMessage, http.StatusInternalServerError)
			return
		}

		websession.Values[sessionCreationTimeKey] = string(timeNow)
		websession.Values[sessionKey] = key

		remoteAddr := strings.SplitN(req.RemoteAddr, ":", 2)
		if len(remoteAddr) != 2 { // TODO: ctrl+f RemoteAddr and move this routine to helper
			log.Errorf("Format of IP address %s (should be IP:PORT) not recognized", req.RemoteAddr)
			http.Error(res, genericErrorMessage, http.StatusInternalServerError)
			return
		}
		websession.Values[ipAddressKey] = remoteAddr[0]

		if err := websession.Save(req, res); err != nil {
			log.Errorf("\"%s\" occurred while trying to save session to browser", err.Error())
			http.Error(res, genericErrorMessage, http.StatusInternalServerError)
			return
		}

		// redirect to dashboard
		http.Redirect(res, req, "/", http.StatusSeeOther)
		return
	}

	// Render login page (shows up on non-POST requests)
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "VCH"
	}
	loginPageData := &LoginPageData{
		Hostname:     hostName,
		SystemTime:   time.Now().Format(time.UnixDate),
		InvalidLogin: invalidLoginMessage,
	}

	tmpl, err := template.ParseFiles("auth.html")
	err = tmpl.ExecuteTemplate(res, "auth.html", loginPageData)
	if err != nil {
		log.Errorf("Error parsing template: %s", err)
		http.Error(res, genericErrorMessage, http.StatusInternalServerError)
		return
	}
}

func (s *server) serve() error {
	defer trace.End(trace.Begin(""))

	s.mux = http.NewServeMux()

	// unauthenticated routes
	// these assets bypass authentication & are world-readable
	s.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css/"))))
	s.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images/"))))
	s.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir("fonts/"))))

	s.Handle(loginPagePath, http.HandlerFunc(s.loginPage))

	// authenticated routes
	// tar of appliance system logs
	s.Authenticated("/logs.tar.gz", s.tarDefaultLogs)
	s.Authenticated("/logs.zip", s.zipDefaultLogs)

	// tar of appliance system logs + container logs
	s.Authenticated("/container-logs.tar.gz", s.tarContainerLogs)
	s.Authenticated("/container-logs.zip", s.zipContainerLogs)

	// get single log file (no tail)
	s.Authenticated("/logs/", func(w http.ResponseWriter, r *http.Request) {
		file := strings.TrimPrefix(r.URL.Path, "/logs/")
		log.Debugf("writing contents for %s", file)
		writeLogFiles(w, r, file, true)
	})

	// get single log file (with tail)
	s.Authenticated("/logs/tail/", func(w http.ResponseWriter, r *http.Request) {
		file := strings.TrimPrefix(r.URL.Path, "/logs/tail/")
		log.Debugf("writing contents for %s", file)
		writeLogFiles(w, r, file, false)
		s.tailFiles(w, r, []string{filepath.Join(logFileDir, file)})
	})

	s.Authenticated("/logout", s.logoutHandler)
	s.Authenticated("/", s.index)
	server := &http.Server{
		Handler: s.mux,
	}

	return server.Serve(s.l)
}

func (s *server) stop() error {
	defer trace.End(trace.Begin(""))

	if s.l != nil {
		err := s.l.Close()
		s.l = nil
		return err
	}

	return nil
}

// logout handler expires the user's session cookie by setting its creation time to the beginning of time
func (s *server) logoutHandler(res http.ResponseWriter, req *http.Request) {
	// #nosec: Errors unhandled.
	websession, _ := s.uss.cookies.Get(req, sessionCookieKey)
	// ignore parsing/marshalling errors because we're parsing a hardcoded beginning-of-time string
	websession.Values[sessionCreationTimeKey] = beginningOfTime
	if err := websession.Save(req, res); err != nil {
		http.Error(res, "Failed to expire user session", http.StatusInternalServerError)
		return
	}
	s.uss.Delete(websession.Values[sessionKey].(string))
	http.Redirect(res, req, "/authentication", http.StatusTemporaryRedirect)
}

func (s *server) bundleContainerLogs(res http.ResponseWriter, req *http.Request, f format) {
	defer trace.End(trace.Begin(""))
	logrotateLock := filelock.NewFileLock(filelock.LogRotateLockName)
	if err := logrotateLock.Acquire(); err != nil {
		log.Errorf("Failed to acquire logrotate lock: %s", err)
	} else {
		defer func() { logrotateLock.Release() }()
	}

	readers := configureReaders()
	c, err := s.getSessionFromRequest(context.Background(), req)
	if err != nil {
		log.Errorf("Failed to get vSphere session while bundling container logs due to error: %s", err.Error())
		http.Redirect(res, req, "/logout", http.StatusTemporaryRedirect)
		return
	}

	logs, err := findDatastoreLogs(c)
	if err != nil {
		log.Warningf("error searching datastore: %s", err)
	} else {
		for key, rdr := range logs {
			readers[key] = rdr
		}
	}

	logs, err = findDiagnosticLogs(c)
	if err != nil {
		log.Warningf("error collecting diagnostic logs: %s", err)
	} else {
		for key, rdr := range logs {
			readers[key] = rdr
		}
	}

	s.bundleLogs(res, req, readers, f)
}

func (s *server) tarDefaultLogs(res http.ResponseWriter, req *http.Request) {
	defer trace.End(trace.Begin(""))

	s.bundleLogs(res, req, configureReaders(), formatTGZ)
}

func (s *server) zipDefaultLogs(res http.ResponseWriter, req *http.Request) {
	defer trace.End(trace.Begin(""))

	s.bundleLogs(res, req, configureReaders(), formatZip)
}

func (s *server) bundleLogs(res http.ResponseWriter, req *http.Request, readers map[string]entryReader, f format) {
	defer trace.End(trace.Begin(""))

	var err error
	if f == formatTGZ {
		res.Header().Set("Content-Type", "application/x-gzip")
		z := gzip.NewWriter(res)
		defer z.Close()
		err = tarEntries(readers, z)
	} else if f == formatZip {
		res.Header().Set("Content-Type", "application/zip")
		z := zip.NewWriter(res)
		defer z.Close()
		err = zipEntries(readers, z)
	}

	if err != nil {
		log.Errorf("Error bundling logs: %s", err)
	}
}

func (s *server) tarContainerLogs(res http.ResponseWriter, req *http.Request) {
	s.bundleContainerLogs(res, req, formatTGZ)
}

func (s *server) zipContainerLogs(res http.ResponseWriter, req *http.Request) {
	s.bundleContainerLogs(res, req, formatZip)
}

func (s *server) tailFiles(res http.ResponseWriter, req *http.Request, names []string) {
	defer trace.End(trace.Begin(""))

	cc := res.(http.CloseNotifier).CloseNotify()

	fw := &flushWriter{
		f: res.(http.Flusher),
		w: res,
	}

	done := make(chan bool)
	for _, file := range names {
		go tailFile(fw, file, &done)
	}

	<-cc
	for range names {
		done <- true
	}
}

func (s *server) index(res http.ResponseWriter, req *http.Request) {
	defer trace.End(trace.Begin(""))
	ctx := context.Background()
	sess, err := s.getSessionFromRequest(ctx, req)
	if err != nil {
		log.Errorf("While loading index page got %s looking up a vSphere session", err.Error())
		http.Redirect(res, req, "/logout", http.StatusTemporaryRedirect)
		return
	}
	v := vicadmin.NewValidator(ctx, &vchConfig, sess)
	if sess == nil {
		// We're unable to connect to vSphere, so display an error message
		v.VCHIssues = template.HTML("<span class=\"error-message\">We're having some trouble communicating with vSphere. <a href=\"/logout\">Logging in again</a> may resolve the issue.</span>\n")
	}

	tmpl, err := template.ParseFiles("dashboard.html")
	err = tmpl.ExecuteTemplate(res, "dashboard.html", v)
	if err != nil {
		log.Errorf("Error parsing template: %s", err)
	}
}
