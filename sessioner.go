package session

import (
	"encoding/gob"
	"errors"
	"log"
	"net/url"

	"github.com/insionng/macross"
)

var GlobalManager *Manager

var defaultOtions = Options{"memory", `{"cookieName":"MacrossSessionId","gcLifetime":3600}`}

//var defaultOtions = Options{"file", `{"cookieName":"MacrossSessionId","gcLifetime":3600,"providerConfig":"./data/session"}`}

//var defaultOtions = Options{"redis", `{"cookieName":"MacrossSessionId","gcLifetime":3600,"providerConfig":"127.0.0.1:6379"}`}

const (
	CONTEXT_SESSION_KEY = "_SESSION_STORE"
	COOKIE_FLASH_KEY    = "_COOKIE_FLASH"
	CONTEXT_FLASH_KEY   = "_FLASH_VALUE"
	SESSION_FLASH_KEY   = "_SESSION_FLASH"
	SESSION_INPUT_KEY   = "_SESSION_INPUT"
)

// Store is the interface that contains all data for one session process with specific ID.
type Store interface {
	RawStore
	// Read returns raw session store by session ID.
	Read(string) (macross.RawStore, error)
	// Destory deletes a session.
	Destory(*macross.Context) error
	// RegenerateId regenerates a session store from old session ID to new one.
	RegenerateId(*macross.Context) (macross.RawStore, error)
	// Count counts and returns number of sessions.
	Count() int
	// GC calls GC to clean expired sessions.
	GC()
}

type store struct {
	RawStore
	*Manager
}

var _ Store = &store{}

type Options struct {
	Provider string
	Config   string
}

func init() {
	gob.Register(url.Values{})
}

// setup 初始化并设置session配置
func setup(op ...Options) error {
	option := defaultOtions
	if len(op) > 0 {
		option = op[0]
	}

	if len(option.Provider) == 0 {
		option.Provider = defaultOtions.Provider
		option.Config = defaultOtions.Config
	}

	log.Println("Macross session config:", option)

	var err error
	GlobalManager, err = NewManager(option.Provider, option.Config)
	if err != nil {
		return err
	}
	go GlobalManager.GC()

	return nil
}

// Sessioner Macross session 中间件
func Sessioner(op ...Options) macross.Handler {
	if GlobalManager == nil {
		if err := setup(op...); err != nil {
			log.Fatalln("Sessioner() setup() errors:", err)
		}
	}
	return func(c *macross.Context) error {
		if GlobalManager == nil {
			return errors.New("session manager not found, use session middleware but not init ?")
		}

		sess, err := GlobalManager.Start(c)
		if err != nil {
			return err
		}

		c.Session = store{
			RawStore: sess,
			Manager:  GlobalManager,
		}
		c.Flash = new(macross.Flash)
		c.Set(CONTEXT_FLASH_KEY, Flash{})

		flashVals := url.Values{}
		flashIf := sess.Get(SESSION_FLASH_KEY)
		if flashIf != nil {
			vals, _ := url.QueryUnescape(flashIf.(string))
			flashVals, _ = url.ParseQuery(vals)
			if len(flashVals) > 0 {
				flash := macross.Flash{}
				flash.ErrorMsg = flashVals.Get("error")
				flash.WarningMsg = flashVals.Get("warning")
				flash.InfoMsg = flashVals.Get("info")
				flash.SuccessMsg = flashVals.Get("success")

				c.Flash = &flash
				// flash先暂存到context里面
				c.Set(CONTEXT_FLASH_KEY, flash)

			}
		}

		f := NewFlash(c)
		c.Flash = f
		c.Set(CONTEXT_SESSION_KEY, c.Session)

		defer func() {
			//log.Println("save session", sess)
			sess.Set(SESSION_FLASH_KEY, url.QueryEscape(f.Encode()))
			sess.Release(c)
		}()

		return c.Next()
	}
}

func GetStore(c *macross.Context) Store {
	store := c.Get(CONTEXT_SESSION_KEY)
	if store != nil {
		if s, okay := store.(Store); okay {
			return s
		}
	}
	return nil
}

func GetFlash(c *macross.Context) *macross.Flash {
	if store := GetStore(c); store != nil {
		if tmp := store.Get(SESSION_FLASH_KEY); tmp != nil {
			if flash, okay := tmp.(*macross.Flash); okay {
				return flash
			}
		}
	}
	return NewFlash(c)
}

func FlashValue(c *macross.Context) Flash {
	if tmp := c.Get(CONTEXT_FLASH_KEY); tmp != nil {
		return tmp.(Flash)
	}
	return Flash{}
}

func SaveInput(c *macross.Context) {
	if store := GetStore(c); store != nil {
		store.Set(SESSION_INPUT_KEY, url.Values(c.FormParams()))
	}
}

func GetInput(c *macross.Context) url.Values {
	if store := GetStore(c); store != nil {
		input := store.Get(SESSION_INPUT_KEY)
		if input != nil {
			return input.(url.Values)
		}
	}
	return url.Values{}
}

func CleanInput(c *macross.Context) {
	if store := GetStore(c); store != nil {
		store.Set(SESSION_INPUT_KEY, url.Values{})
	}
}

func NewFlash(ctx *macross.Context) *macross.Flash {
	return &macross.Flash{ctx, url.Values{}, "", "", "", ""}
}

type Flash struct {
	url.Values
	ErrorMsg, WarningMsg, InfoMsg, SuccessMsg string
}

func (f *Flash) set(name, msg string) {
	f.Set(name, msg)
}

func (f *Flash) Error(msg string) {
	f.ErrorMsg = msg
	f.set("error", msg)
}

func (f *Flash) Warning(msg string) {
	f.WarningMsg = msg
	f.set("warning", msg)
}

func (f *Flash) Info(msg string) {
	f.InfoMsg = msg
	f.set("info", msg)
}

func (f *Flash) Success(msg string) {
	f.SuccessMsg = msg
	f.set("success", msg)
}
