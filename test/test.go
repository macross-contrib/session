package main

import (
	"github.com/insionng/macross"
	"github.com/insionng/macross/recover"
	"github.com/macross-contrib/session"
	_ "github.com/macross-contrib/session/redis"
	"log"
)

func main() {

	v := macross.New()
	v.Use(recover.Recover())
	//v.Use(session.Sessioner(session.Options{"file", `{"cookieName":"MacrossSessionId","gclifetime":3600,"ProviderConfig":"./data/session"}`}))
	v.Use(session.Sessioner(session.Options{"redis", `{"cookieName":"MacrossSessionId","gcLifetime":3600,"ProviderConfig":"127.0.0.1:6379"}`}))

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

	v.Run(":8080")
}
