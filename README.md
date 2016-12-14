Session
==============

The session package is a Macross session manager. It can use many session providers.

## How to install?

	go get github.com/macross-contrib/session


## What providers are supported?

As of now this session manager support memory, file and Redis .


## How to use it?

First you must import it

	import (
		"github.com/macross-contrib/session"
	)


* Use **memory** as provider:

        session.Options{"memory", `{"cookieName":"MacrossSessionId","gclifetime":3600}`}

* Use **file** as provider, the last param is the path where you want file to be stored:

	    session.Options{"file", `{"cookieName":"MacrossSessionId","gclifetime":3600,"ProviderConfig":"./data/session"}`}

* Use **Redis** as provider, the last param is the Redis conn address,poolsize,password:

		session.Options{"redis", `{"cookieName":"MacrossSessionId","gclifetime":3600,"ProviderConfig":"127.0.0.1:6379,100,macross"}`}

* Use **Cookie** as provider:

		session.Options{"cookie", `{"cookieName":"MacrossSessionId","enableSetCookie":false,"gclifetime":3600,"ProviderConfig":"{\"cookieName\":\"MacrossSessionId\",\"securityKey\":\"beegocookiehashkey\"}"}`}


Finally in the code you can use it like this

```go
package main

import (
	"github.com/insionng/macross"
	"github.com/insionng/macross/recover"
	"github.com/macross-contrib/session"
	//_ "github.com/macross-contrib/session/redis"
	"log"
)

func main() {

	v := macross.New()
	v.Use(recover.Recover())
	v.Use(session.Sessioner(session.Options{"file", `{"cookieName":"MacrossSessionId","gclifetime":3600,"ProviderConfig":"./data/session"}`}))
	//v.Use(session.Sessioner(session.Options{"redis", `{"cookieName":"MacrossSessionId","gcLifetime":3600,"ProviderConfig":"127.0.0.1:6379"}`}))

	v.Get("/get", func(self *macross.Context) error {
		sess := session.GetStore(self)

		value := "nil"
		valueIf := sess.Get("key")
		if valueIf != nil {
			value = valueIf.(string)
		}

		return self.String(value)

	})

	v.Get("/set", func(self *macross.Context) error {
		sess := session.GetStore(self)

		val := self.QueryParam("v")
		if len(val) == 0 {
			val = "value"
		}

		err := sess.Set("key", val)
		if err != nil {
			log.Printf("sess.set %v \n", err)
		}
		return self.String("ok")
	})

	v.Run(":9000")
}
```


## How to write own provider?

When you develop a web app, maybe you want to write own provider because you must meet the requirements.

Writing a provider is easy. You only need to define two struct types
(Session and Provider), which satisfy the interface definition.
Maybe you will find the **memory** provider is a good example.

	type SessionStore interface {
		Set(key, value interface{}) error     //set session value
		Get(key interface{}) interface{}      //get session value
		Delete(key interface{}) error         //delete session value
		SessionID() string                    //back current sessionID
		SessionRelease(w http.ResponseWriter) // release the resource & save data to provider & return the data
		Flush() error                         //delete all data
	}

	type Provider interface {
		SessionInit(gclifetime int64, config string) error
		SessionRead(sid string) (SessionStore, error)
		SessionExist(sid string) bool
		SessionRegenerate(oldsid, sid string) (SessionStore, error)
		SessionDestroy(sid string) error
		SessionAll() int //get all active session
		SessionGC()
	}


## LICENSE

MIT License
