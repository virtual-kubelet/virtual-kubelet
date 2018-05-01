// Copyright 2016 VMware, Inc. All Rights Reserved.
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
	"fmt"
	"sync"
	"time"

	"context"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"

	"github.com/vmware/vic/pkg/vsphere/session"
)

// UserSession holds a user's session metadata
type UserSession struct {
	id      string
	created time.Time
	config  *session.Config
	vsphere *session.Session
}

// UserSessionStore holds and manages user sessions
type UserSessionStore struct {
	mutex    sync.RWMutex
	sessions map[string]*UserSession
	ticker   *time.Ticker
	cookies  *sessions.CookieStore
}

type UserSessionStorer interface {
	Add(id string, config *session.Config, vs *session.Session) *UserSession
	Delete(id string)
	VSphere(id string) (vSphereSession *session.Session, err error)
	UserSession(id string) *UserSession
}

// Add a session. VS may be nil if host is plain ESX
func (u *UserSessionStore) Add(id string, config *session.Config, vs *session.Session) *UserSession {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	sess := &UserSession{
		id:      id,
		created: time.Now(),
		// TODO strip out config cause it's not needed anymore, but shows up in a number of places
		config:  config,
		vsphere: vs,
	}
	u.sessions[id] = sess
	return sess
}

func (u *UserSessionStore) Delete(id string) {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	delete(u.sessions, id)
}

// Grabs the UserSession metadata object and doesn't establish a connection to vSphere
func (u *UserSessionStore) UserSession(id string) *UserSession {
	u.mutex.RLock()
	defer u.mutex.RUnlock()
	return u.sessions[id]
}

// Returns a vSphere session object. Caller responsible for error handling/logout
func (u *UserSessionStore) VSphere(ctx context.Context, id string) (*session.Session, error) {
	us := u.UserSession(id)
	if us == nil {
		return nil, fmt.Errorf("User session with unique ID %s does not exist", id)
	}
	if us.vsphere == nil {
		return nil, fmt.Errorf("No vSphere session found for user: %s", id)
	}

	vsphus, err := us.vsphere.SessionManager.UserSession(ctx)
	if err != nil || vsphus == nil {
		if err != nil {
			log.Warnf("Failed to validate user %s session: %v", id, err)
			return nil, err
		}
		return nil, fmt.Errorf("User %s session has expired", id)
	}
	log.Infof("Found vSphere session for vicadmin usersession %s", id)
	return us.vsphere, nil
}

// reaper takes abandoned sessions to a farm upstate so they don't build up forever
func (u *UserSessionStore) reaper() {
	for range u.ticker.C {
		for id, session := range u.sessions {
			if time.Since(session.created) > sessionExpiration {
				u.Delete(id)
			}
		}
	}
}

// NewUserSessionStore creates & initializes a UserSessionStore and starts a session reaper in the background
func NewUserSessionStore() *UserSessionStore {
	u := &UserSessionStore{
		sessions: make(map[string]*UserSession),
		ticker:   time.NewTicker(time.Minute * 10),
		mutex:    sync.RWMutex{},
		cookies: sessions.NewCookieStore(
			[]byte(securecookie.GenerateRandomKey(64)),
			[]byte(securecookie.GenerateRandomKey(32))),
	}
	u.cookies.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   int(sessionExpiration.Seconds()),
		Secure:   true,
		HttpOnly: true,
	}
	go u.reaper()
	return u
}
