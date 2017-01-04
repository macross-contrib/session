package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
	//"log"

	"github.com/insionng/macross"
)

// RawStore contains all data for one session process with specific id.
type RawStore interface {
	Set(key, value interface{}) error   //set session value
	Get(key interface{}) interface{}    //get session value
	Delete(key interface{}) error       //delete session value
	ID() string                         //back current sessionID
	Release(ctx *macross.Context) error // release the resource & save data to provider & return the data
	// Flush deletes all session data.
	Flush() error
}

// Provider contains global session methods and saved SessionStores.
// it can operate a SessionStore by its id.
type Provider interface {
	Init(gcLifetime int64, config string) error
	Read(sid string) (macross.RawStore, error)
	Exist(sid string) bool
	Regenerate(oldsid, sid string) (macross.RawStore, error)
	Destory(sid string) error
	Count() int //get all active session
	GC()
}

var provides = make(map[string]Provider)

// Register makes a session provide available by the provided name.
// If Register is called twice with the same name or if driver is nil,
// it panics.
func Register(name string, provide Provider) {
	if provide == nil {
		panic("session: Register provide is nil")
	}
	if _, dup := provides[name]; dup {
		panic("session: Register called twice for provider " + name)
	}
	provides[name] = provide
}

type managerConfig struct {
	CookieName      string `json:"cookieName"`
	EnableSetCookie bool   `json:"enableSetCookie,omitempty"`
	GcLifetime      int64  `json:"gcLifetime"`
	MaxLifetime     int64  `json:"maxLifetime"`
	Secure          bool   `json:"secure"`
	CookieLifetime  int    `json:"cookieLifetime"`
	ProviderConfig  string `json:"providerConfig"`
	Domain          string `json:"domain"`
	SessionIDLength int64  `json:"sessionIDLength"`
}

// Manager contains Provider and its configuration.
type Manager struct {
	provider Provider
	config   *managerConfig
}

// NewManager Create new Manager with provider name and json config string.
// provider name:
// 1. cookie
// 2. file
// 3. memory
// 4. redis
// 5. mysql
// json config:
// 1. is https  default false
// 2. hashfunc  default sha1
// 3. hashkey default beegosessionkey
// 4. maxage default is none
func NewManager(provideName, config string) (*Manager, error) {
	provider, ok := provides[provideName]
	if !ok {
		return nil, fmt.Errorf("session: unknown provide %q (forgotten import?)", provideName)
	}
	cf := new(managerConfig)
	cf.EnableSetCookie = true
	err := json.Unmarshal([]byte(config), cf)
	if err != nil {
		return nil, err
	}
	if cf.MaxLifetime == 0 {
		cf.MaxLifetime = cf.GcLifetime
	}
	err = provider.Init(cf.MaxLifetime, cf.ProviderConfig)
	if err != nil {
		return nil, err
	}

	if cf.SessionIDLength == 0 {
		cf.SessionIDLength = 16
	}

	return &Manager{
		provider,
		cf,
	}, nil
}

// getSid retrieves session identifier from HTTP Request.
// First try to retrieve id by reading from cookie, session cookie name is configurable,
// if not exist, then retrieve id from querying parameters.
//
// error is not nil when there is anything wrong.
// sid is empty when need to generate a new session id
// otherwise return an valid session id.
func (manager *Manager) getSid(ctx *macross.Context) (string, error) {
	//log.Println("get cookie name", manager.config.CookieName)
	cookie, errs := ctx.Cookie(manager.config.CookieName)

	if errs != nil || cookie.Value() == "" {
		//log.Println("read from query")
		sid := ctx.FormValue(manager.config.CookieName)
		return sid, nil
	}

	// HTTP Request contains cookie for sessionid info.
	return url.QueryUnescape(cookie.Value())
}

// Start generate or read the session id from http request.
// if session id exists, return SessionStore with this id.
func (manager *Manager) Start(ctx *macross.Context) (session macross.RawStore, err error) {
	sid, errs := manager.getSid(ctx)
	if errs != nil {
		return nil, errs
	}

	//log.Println("start sid", sid)

	if sid != "" && manager.provider.Exist(sid) {
		//log.Println("sid exists")
		return manager.provider.Read(sid)
	}

	//log.Println("sid not exists")

	// Generate a new session
	sid, errs = manager.sessionID()
	if errs != nil {
		return nil, errs
	}

	session, err = manager.provider.Read(sid)
	cookie := new(macross.Cookie)
	cookie.SetName(manager.config.CookieName)
	cookie.SetValue(url.QueryEscape(sid))
	cookie.SetPath("/")
	cookie.SetHTTPOnly(true)
	cookie.SetSecure(manager.isSecure(ctx))
	cookie.SetDomain(manager.config.Domain)

	if manager.config.CookieLifetime > 0 {
		// cookie.MaxAge = manager.config.CookieLifetime
		cookie.SetExpire(time.Now().Add(time.Duration(manager.config.CookieLifetime)))
	}
	if manager.config.EnableSetCookie {
		ctx.SetCookie(cookie)

	}

	// r.AddCookie(cookie)

	return
}

// Read returns raw session store by session ID.
func (manager *Manager) Read(sid string) (rawStore macross.RawStore, err error) {
	rawStore, err = manager.provider.Read(sid)
	return
}

// Count counts and returns number of sessions.
func (m *Manager) Count() int {
	return m.provider.Count()
}

// GC Start session gc process.
// it can do gc in times after gc lifetime.
func (manager *Manager) GC() {
	manager.provider.GC()
	time.AfterFunc(time.Duration(manager.config.GcLifetime)*time.Second, func() { manager.GC() })
}

// RegenerateId Regenerate a session id for this SessionStore who's id is saving in http request.
func (manager *Manager) RegenerateId(ctx *macross.Context) (session macross.RawStore, err error) {
	sid, err := manager.sessionID()
	if err != nil {
		return
	}
	var c *macross.Cookie
	cookie, err := ctx.Cookie(manager.config.CookieName)
	if err != nil || cookie.Value() == "" {
		//delete old cookie
		session, _ = manager.provider.Read(sid)
		c = new(macross.Cookie)
		c.SetName(manager.config.CookieName)
		c.SetValue(url.QueryEscape(sid))
		c.SetPath("/")
		c.SetHTTPOnly(true)
		c.SetSecure(manager.isSecure(ctx))
		c.SetDomain(manager.config.Domain)

	} else {
		oldsid, _ := url.QueryUnescape(cookie.Value())
		session, _ = manager.provider.Regenerate(oldsid, sid)

		c = new(macross.Cookie)
		c.SetName(cookie.Name())
		c.SetValue(url.QueryEscape(sid))
		c.SetPath("/")
		c.SetHTTPOnly(true)
		c.SetSecure(cookie.Secure())
		c.SetDomain(cookie.Domain())
	}
	if manager.config.CookieLifetime > 0 {
		// cookie.MaxAge = manager.config.CookieLifetime
		c.SetExpire(time.Now().Add(time.Duration(manager.config.CookieLifetime)))

	}
	if manager.config.EnableSetCookie {
		ctx.SetCookie(c)

	}
	// r.AddCookie(c)
	return
}

// Destory deletes a session by given ID.
func (m *Manager) Destory(self *macross.Context) error {

	var sid string
	c, e := self.Cookie(m.config.CookieName)
	if !(e != nil) {
		sid = c.Value()
	}

	if len(sid) == 0 {
		return nil
	}

	if err := m.provider.Destory(sid); err != nil {
		return err
	}

	cookie := new(macross.Cookie)
	cookie.SetName(m.config.CookieName)
	cookie.SetPath("/")
	cookie.SetHTTPOnly(true)
	cookie.SetExpire(time.Now())
	self.SetCookie(cookie)
	return nil
}

// SetSecure Set cookie with https.
func (manager *Manager) SetSecure(secure bool) {
	manager.config.Secure = secure
}

func (manager *Manager) sessionID() (string, error) {
	b := make([]byte, manager.config.SessionIDLength)
	n, err := rand.Read(b)
	if n != len(b) || err != nil {
		return "", fmt.Errorf("Could not successfully read from the system CSPRNG.")
	}
	return hex.EncodeToString(b), nil
}

// Set cookie with https.
func (manager *Manager) isSecure(ctx *macross.Context) bool {
	if !manager.config.Secure {
		return false
	}
	if ctx.Scheme() != "" {
		return ctx.Scheme() == "https"
	}

	return false
	// if req.TLS == nil {
	// 	return false
	// }
	// return true
}
